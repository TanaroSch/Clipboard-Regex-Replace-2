// ==== internal/app/app.go ====
package app

import (
	"fmt"
	"log"
	"os"      // Import os package
	"os/exec"
	"path/filepath" // Import filepath package
	"runtime"
	"syscall" // Import syscall
	"unsafe"  // Import unsafe

	"github.com/TanaroSch/clipboard-regex-replace/internal/clipboard"
	"github.com/TanaroSch/clipboard-regex-replace/internal/config"
	"github.com/TanaroSch/clipboard-regex-replace/internal/hotkey"
	"github.com/TanaroSch/clipboard-regex-replace/internal/resources"
	"github.com/TanaroSch/clipboard-regex-replace/internal/ui"
)

// --- Add Windows API constants ---
const (
	SW_SHOWNORMAL = 1
)

// --- ShellExecute func definition (Load DLL and Proc) ---
var (
	// Only attempt to load shell32 on Windows platforms
	shell32           *syscall.LazyDLL
	procShellExecuteW *syscall.LazyProc
)

// Initialize Windows-specific DLLs and Procs
func init() {
	if runtime.GOOS == "windows" {
		shell32 = syscall.NewLazyDLL("shell32.dll")
		procShellExecuteW = shell32.NewProc("ShellExecuteW")
	}
}

// ShellExecute simplifies calling the Windows ShellExecuteW API
// Note: This function will only work correctly on Windows.
func ShellExecute(hwnd uintptr, verb, file, params, dir string, showCmd int32) (err error) {
	// Prevent panic if somehow called on non-windows
	if runtime.GOOS != "windows" {
		return fmt.Errorf("ShellExecute called on non-Windows OS")
	}

	// Convert Go strings to UTF-16 pointers required by Windows API
	lpVerb, err := syscall.UTF16PtrFromString(verb)
	if err != nil {
		return fmt.Errorf("failed to convert verb to UTF16Ptr: %w", err)
	}
	lpFile, err := syscall.UTF16PtrFromString(file)
	if err != nil {
		return fmt.Errorf("failed to convert file path to UTF16Ptr: %w", err)
	}
	// Handle potentially empty strings for params and dir safely
	var lpParams *uint16
	if params != "" {
		lpParams, err = syscall.UTF16PtrFromString(params)
		if err != nil {
			return fmt.Errorf("failed to convert params to UTF16Ptr: %w", err)
		}
	}
	var lpDir *uint16
	if dir != "" {
		lpDir, err = syscall.UTF16PtrFromString(dir)
		if err != nil {
			return fmt.Errorf("failed to convert dir to UTF16Ptr: %w", err)
		}
	}

	// Call the ShellExecuteW procedure
	ret, _, callErr := procShellExecuteW.Call(
		hwnd,
		uintptr(unsafe.Pointer(lpVerb)),
		uintptr(unsafe.Pointer(lpFile)),
		uintptr(unsafe.Pointer(lpParams)), // Use potentially nil pointer
		uintptr(unsafe.Pointer(lpDir)),    // Use potentially nil pointer
		uintptr(showCmd),
	)

	// Values > 32 indicate success (per ShellExecute documentation)
	// https://docs.microsoft.com/en-us/windows/win32/api/shellapi/nf-shellapi-shellexecutew
	// Instance handle is returned on success, error code otherwise.
	if ret <= 32 {
		// Combine the return code with any error from the Call itself
		if callErr != nil && callErr.Error() != "The operation completed successfully." {
			// Sometimes Call returns an error object even on "success" according to docs
			err = fmt.Errorf("ShellExecuteW failed with return code %d and call error: %w", ret, callErr)
		} else {
			err = fmt.Errorf("ShellExecuteW failed with return code %d", ret)
		}
	} else {
		// Success, clear any potential "success" error message from Call
		err = nil
	}

	return err
}

// Application represents the main application
type Application struct {
	config           *config.Config
	version          string
	clipboardManager *clipboard.Manager
	hotkeyManager    *hotkey.Manager
	systrayManager   *ui.SystrayManager
	iconData         []byte
}

// New creates a new application instance
func New(cfg *config.Config, version string) *Application {
	app := &Application{
		config:  cfg,
		version: version,
	}

	// Load embedded icon
	var err error
	app.iconData, err = resources.GetIcon()
	if err != nil {
		log.Printf("Warning: Failed to load embedded icon: %v", err)
	}

	// Initialize global notifications
	ui.InitGlobalNotifications(cfg.UseNotifications, "Clipboard Regex Replace", app.iconData)

	// Create clipboard manager
	app.clipboardManager = clipboard.NewManager(cfg, app.onRevertStatusChange)

	// Create hotkey manager
	app.hotkeyManager = hotkey.NewManager(cfg, app.onHotkeyTriggered, app.onRevertHotkey)

	// Create systray manager
	app.systrayManager = ui.NewSystrayManager(
		cfg,
		version,
		app.iconData,
		app.onReloadConfig,
		app.onRestartApplication,
		app.onQuit,
		app.onRevertMenuItem,
		app.onOpenConfigFile, // Pass the new method
	)

	return app
}

// Run starts the application
func (a *Application) Run() {
	// Register hotkeys
	if err := a.hotkeyManager.RegisterAll(); err != nil {
		log.Printf("Warning: Failed to register some hotkeys: %v", err)
		ui.ShowNotification("Hotkey Registration Issue",
			fmt.Sprintf("Some hotkeys could not be registered: %v", err))
	}

	// Start systray
	a.systrayManager.Run()
}

// onHotkeyTriggered is called when a hotkey is pressed
func (a *Application) onHotkeyTriggered(hotkeyStr string, isReverse bool) {
	message := a.clipboardManager.ProcessClipboard(hotkeyStr, isReverse)
	if message != "" {
		ui.ShowNotification("Clipboard Updated", message)
	}
}

// onRevertHotkey is called when the revert hotkey is pressed
func (a *Application) onRevertHotkey() {
	if a.clipboardManager.RestoreOriginalClipboard() {
		ui.ShowNotification("Clipboard Reverted", "Original clipboard content has been restored.")
	}
}

// onRevertMenuItem is called when the revert menu item is clicked
func (a *Application) onRevertMenuItem() {
	a.onRevertHotkey()
}

// onRevertStatusChange is called when revert status changes
func (a *Application) onRevertStatusChange(canRevert bool) {
	a.systrayManager.UpdateRevertStatus(canRevert)
}

// onReloadConfig is called when the reload config menu item is clicked
func (a *Application) onReloadConfig() {
	log.Println("Reloading configuration...")

	// Store the original number of profiles and their names for comparison
	originalProfileCount := len(a.config.Profiles)
	originalProfileNames := make(map[string]bool)
	for _, profile := range a.config.Profiles {
		originalProfileNames[profile.Name] = true
	}

	// Store current enabled status of profiles to preserve user's runtime choices
	enabledStatus := make(map[string]bool)
	for _, profile := range a.config.Profiles {
		enabledStatus[profile.Name] = profile.Enabled
	}

	// Load the updated configuration
	configPath := a.config.GetConfigPath() // Get path before potentially changing config object
	if configPath == "" {
		log.Println("Cannot reload config: original config path is empty.")
		ui.ShowNotification("Configuration Error", "Cannot determine config file path to reload.")
		return
	}
	newConfig, err := config.Load(configPath)
	if err != nil {
		log.Printf("Error reloading configuration from '%s': %v", configPath, err)
		ui.ShowNotification("Configuration Error",
			"Failed to reload configuration. Check logs for details.")
		return
	}

	// Update the application's config reference
	a.config = newConfig

	// Restore enabled status for profiles that still exist
	// This preserves the user's runtime choices even after a config reload
	for i, profile := range a.config.Profiles {
		if enabled, exists := enabledStatus[profile.Name]; exists {
			a.config.Profiles[i].Enabled = enabled
		}
	}

	// Check if profile structure has changed
	profileStructureChanged := originalProfileCount != len(a.config.Profiles)

	if !profileStructureChanged {
		// Check if any profile names have changed
		newProfileNames := make(map[string]bool)
		for _, profile := range a.config.Profiles {
			newProfileNames[profile.Name] = true
		}
		for name := range originalProfileNames {
			if !newProfileNames[name] {
				profileStructureChanged = true
				break
			}
		}
		if !profileStructureChanged {
			for name := range newProfileNames {
				if !originalProfileNames[name] {
					profileStructureChanged = true
					break
				}
			}
		}
	}

	log.Println("Configuration reloaded successfully.")

	// Re-register hotkeys (ensure hotkey manager uses the *updated* config)
	a.hotkeyManager = hotkey.NewManager(a.config, a.onHotkeyTriggered, a.onRevertHotkey) // Recreate manager with new config
	if err := a.hotkeyManager.RegisterAll(); err != nil {
		log.Printf("Warning: Failed to register some hotkeys after reload: %v", err)
		ui.ShowNotification("Hotkey Registration Issue",
			fmt.Sprintf("Some hotkeys could not be registered after reload: %v", err))
	}

	// Update UI elements dependent on config
	a.systrayManager.UpdateConfig(a.config) // Add an UpdateConfig method to SystrayManager if needed

	if profileStructureChanged {
		// If profile structure changed, we ideally need to rebuild the menu.
		// Systray library might not support dynamic menu rebuild easily. Restart is safest.
		log.Println("Profile structure changed significantly. Restarting application is recommended.")
		ui.ShowNotification("Configuration Reloaded",
			"Profile structure changed. Please Restart Application via menu to fully refresh UI.")
		// Optionally trigger restart automatically:
		// a.onRestartApplication()

	} else {
		// For simple config changes, just update in memory
		ui.ShowNotification("Configuration Reloaded",
			"Configuration updated successfully. Hotkeys have been refreshed.")
	}
}

// onRestartApplication is called when the restart application menu item is clicked
func (a *Application) onRestartApplication() {
	ui.RestartApplication()
}

// onQuit is called when the quit menu item is clicked
func (a *Application) onQuit() {
	// Unregister all hotkeys
	a.hotkeyManager.UnregisterAll()
}

// onOpenConfigFile is called when the open config menu item is clicked
func (a *Application) onOpenConfigFile() {
	configPath := a.config.GetConfigPath()
	if configPath == "" {
		log.Println("Error: Config path is empty, cannot open file.")
		ui.ShowNotification("Error Opening File", "Configuration file path is not set.")
		return
	}
	log.Printf("Initial config path from config object: %s", configPath) // Log initial path

	// Get absolute path for robustness
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		log.Printf("Warning: Failed to get absolute path for '%s': %v. Proceeding with original path.", configPath, err)
		// Use the original path if Abs fails
		absPath = configPath
	} else {
		log.Printf("Absolute config path resolved to: %s", absPath)
		configPath = absPath // Use the absolute path
	}

	// Check if the file exists before trying to open it
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Printf("Error: Config file does not exist at path: %s", configPath)
		ui.ShowNotification("Error Opening File", fmt.Sprintf("Config file not found: %s", configPath))
		return
	} else if err != nil {
		// Log other stat errors (e.g., permission denied) but attempt to open anyway
		log.Printf("Error checking config file status at path '%s': %v", configPath, err)
	} else {
		log.Printf("Config file exists at: %s", configPath)
	}

	log.Printf("Attempting to open config file using helper function with path: %s", configPath)
	err = openFileInDefaultEditor(configPath)
	if err != nil {
		// Error logging now happens inside openFileInDefaultEditor or is returned
		log.Printf("Final error after trying open methods: %v", err)
		ui.ShowNotification("Error Opening File", fmt.Sprintf("Could not open config file: %v", err))
	}
}

// openFileInDefaultEditor attempts to open the specified file path
// using the operating system's default application.
func openFileInDefaultEditor(filePath string) error {
	var cmd *exec.Cmd
	var err error

	log.Printf("Executing openFileInDefaultEditor for path: %s on OS: %s", filePath, runtime.GOOS)

	switch runtime.GOOS {
	case "windows":
		// --- Method 1: Try ShellExecuteW API (most native) ---
		log.Println("Windows: Attempting method 1: ShellExecuteW API")
		// Use "open" verb to open the file with its default application.
		// Pass 0 for hwnd (no parent window), empty strings for params and dir.
		// Use SW_SHOWNORMAL to show the application window normally.
		err = ShellExecute(0, "open", filePath, "", "", SW_SHOWNORMAL)
		if err == nil {
			log.Println("Windows Method 1 (ShellExecuteW) succeeded.")
			return nil // Success!
		}
		log.Printf("Windows Method 1 (ShellExecuteW) failed: %v", err)
		// Store ShellExecute error before trying next method
		shellExecuteErr := err

		// --- Method 2: Fallback to cmd /c start "" "path" ---
		log.Println("Windows: Attempting method 2 (Fallback): cmd /c start \"\" \"<filepath>\"")
		cmdArgs := []string{"/c", "start", "\"\"", "\"" + filePath + "\""}
		cmd = exec.Command("cmd", cmdArgs...)
		log.Printf("Windows Method 2 - Executing: %s %v", cmd.Path, cmd.Args)
		// Use Run instead of Start for the fallback to see if it waits and gives a different error
		err = cmd.Run() // Use Run to wait for completion/error
		if err == nil {
			log.Println("Windows Method 2 (cmd /c start) succeeded.")
			return nil // Success!
		}
		log.Printf("Windows Method 2 (cmd /c start .Run()) failed: %v", err)

		// If both methods failed, return a combined error message
		return fmt.Errorf("failed via ShellExecute (%v) and cmd/start (%v)", shellExecuteErr, err)

	case "darwin": // macOS
		cmd = exec.Command("open", filePath)
		log.Printf("macOS - Executing: %s %v", cmd.Path, cmd.Args)
		// Use Start for non-blocking GUI app launch
		err = cmd.Start()

	default: // Assume Linux or other Unix-like systems
		cmd = exec.Command("xdg-open", filePath)
		log.Printf("Linux/Other - Executing: %s %v", cmd.Path, cmd.Args)
		// Use Start for non-blocking GUI app launch
		err = cmd.Start()
	}

	// Check error for non-Windows or if Windows fallback method failed
	if err != nil {
		log.Printf("Failed to start/run command (%s): %v", cmd.String(), err)
		return fmt.Errorf("failed to start/run command (%s): %w", cmd.String(), err)
	}

	log.Printf("Successfully started/ran command for %s", runtime.GOOS)
	return nil
}