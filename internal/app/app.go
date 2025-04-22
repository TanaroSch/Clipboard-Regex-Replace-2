// ==== internal/app/app.go ====
package app

import (
	"fmt"
	"log"
	"os"
	// "os/exec" // <<< REMOVE THIS IMPORT
	"path/filepath"
	//"runtime"

	"github.com/TanaroSch/clipboard-regex-replace/internal/clipboard"
	"github.com/TanaroSch/clipboard-regex-replace/internal/config"
	"github.com/TanaroSch/clipboard-regex-replace/internal/hotkey"
	"github.com/TanaroSch/clipboard-regex-replace/internal/resources"
	"github.com/TanaroSch/clipboard-regex-replace/internal/ui"
)

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

	var err error
	app.iconData, err = resources.GetIcon()
	if err != nil {
		log.Printf("Warning: Failed to load embedded icon: %v", err)
	}

	ui.InitGlobalNotifications(cfg.UseNotifications, "Clipboard Regex Replace", app.iconData)
	app.clipboardManager = clipboard.NewManager(cfg, app.onRevertStatusChange)
	app.hotkeyManager = hotkey.NewManager(cfg, app.onHotkeyTriggered, app.onRevertHotkey)
	app.systrayManager = ui.NewSystrayManager(
		cfg,
		version,
		app.iconData,
		app.onReloadConfig,
		app.onRestartApplication,
		app.onQuit,
		app.onRevertMenuItem,
		app.onOpenConfigFile,
		app.onViewLastDiffTriggered,
	)

	return app
}

// Run starts the application
func (a *Application) Run() {
	if err := a.hotkeyManager.RegisterAll(); err != nil {
		log.Printf("Warning: Failed to register some hotkeys: %v", err)
		ui.ShowNotification("Hotkey Registration Issue",
			fmt.Sprintf("Some hotkeys could not be registered: %v", err))
	}
	a.systrayManager.Run()
}

// onHotkeyTriggered is called when a hotkey is pressed
func (a *Application) onHotkeyTriggered(hotkeyStr string, isReverse bool) {
	message, changedForDiff := a.clipboardManager.ProcessClipboard(hotkeyStr, isReverse)
	if message != "" {
		ui.ShowNotification("Clipboard Updated", message)
	}
	if a.systrayManager != nil {
		a.systrayManager.UpdateViewLastDiffStatus(changedForDiff)
	}
}

// onViewLastDiffTriggered is called when the "View Last Change Details" menu item is clicked
func (a *Application) onViewLastDiffTriggered() {
	original, modified, ok := a.clipboardManager.GetLastDiff()
	if !ok {
		log.Println("View Last Change Details clicked, but no diff data available.")
		ui.ShowNotification("View Changes", "No changes recorded from the last operation.")
		if a.systrayManager != nil {
			a.systrayManager.UpdateViewLastDiffStatus(false)
		}
		return
	}
	log.Println("View Last Change Details clicked, showing diff viewer.")
	ui.ShowDiffViewer(original, modified)
}


// onRevertHotkey is called when the revert hotkey is pressed
func (a *Application) onRevertHotkey() {
	if a.clipboardManager.RestoreOriginalClipboard() {
		ui.ShowNotification("Clipboard Reverted", "Original clipboard content has been restored.")
		if a.systrayManager != nil {
			a.systrayManager.UpdateViewLastDiffStatus(false)
		}
	}
}

// onRevertMenuItem is called when the revert menu item is clicked
func (a *Application) onRevertMenuItem() {
	a.onRevertHotkey()
}

// onRevertStatusChange is called when revert status changes (from clipboard manager)
func (a *Application) onRevertStatusChange(canRevert bool) {
	if a.systrayManager != nil {
		a.systrayManager.UpdateRevertStatus(canRevert)
	}
}

// onReloadConfig is called when the reload config menu item is clicked
func (a *Application) onReloadConfig() {
	log.Println("Reloading configuration...")

	originalProfileCount := len(a.config.Profiles)
	originalProfileNames := make(map[string]bool)
	for _, profile := range a.config.Profiles {
		originalProfileNames[profile.Name] = true
	}
	enabledStatus := make(map[string]bool)
	for _, profile := range a.config.Profiles {
		enabledStatus[profile.Name] = profile.Enabled
	}

	configPath := a.config.GetConfigPath()
	if configPath == "" {
		log.Println("Cannot reload config: original config path is empty.")
		ui.ShowNotification("Configuration Error", "Cannot determine config file path to reload.")
		return
	}
	newConfig, err := config.Load(configPath)
	if err != nil {
		log.Printf("Error reloading configuration from '%s': %v", configPath, err)
		ui.ShowNotification("Configuration Error",
			fmt.Sprintf("Failed to reload configuration. Check %s for errors.", configPath))
		return
	}

	a.config = newConfig
	for i, profile := range a.config.Profiles {
		if enabled, exists := enabledStatus[profile.Name]; exists {
			a.config.Profiles[i].Enabled = enabled
		}
	}

	profileStructureChanged := originalProfileCount != len(a.config.Profiles)
	if !profileStructureChanged {
		newProfileNames := make(map[string]bool)
		for _, profile := range a.config.Profiles {
			newProfileNames[profile.Name] = true
		}
		for name := range originalProfileNames {
			if !newProfileNames[name] { profileStructureChanged = true; break }
		}
		if !profileStructureChanged {
			for name := range newProfileNames {
				if !originalProfileNames[name] { profileStructureChanged = true; break }
			}
		}
	}

	log.Println("Configuration reloaded successfully.")

	a.hotkeyManager = hotkey.NewManager(a.config, a.onHotkeyTriggered, a.onRevertHotkey)
	if err := a.hotkeyManager.RegisterAll(); err != nil {
		log.Printf("Warning: Failed to register some hotkeys after reload: %v", err)
		ui.ShowNotification("Hotkey Registration Issue",
			fmt.Sprintf("Some hotkeys could not be registered after reload: %v", err))
	}

	a.clipboardManager = clipboard.NewManager(a.config, a.onRevertStatusChange)

	if a.systrayManager != nil {
		a.systrayManager.UpdateConfig(a.config)
		if profileStructureChanged {
			log.Println("Profile structure changed significantly. Restarting application is recommended for full menu update.")
			ui.ShowNotification("Configuration Reloaded",
				"Profile structure changed. Please use 'Restart Application' via menu to fully refresh UI.")
		} else {
			ui.ShowNotification("Configuration Reloaded",
				"Configuration updated successfully. Hotkeys have been refreshed.")
		}
	} else {
		ui.ShowNotification("Configuration Reloaded", "Configuration updated successfully.")
	}
}

// onRestartApplication is called when the restart application menu item is clicked
func (a *Application) onRestartApplication() {
	ui.RestartApplication()
}

// onQuit is called when the quit menu item is clicked
func (a *Application) onQuit() {
	log.Println("Quit requested. Unregistering hotkeys.")
	if a.hotkeyManager != nil {
		a.hotkeyManager.UnregisterAll()
	}
}

// onOpenConfigFile is called when the open config menu item is clicked
func (a *Application) onOpenConfigFile() {
	configPath := a.config.GetConfigPath()
	if configPath == "" {
		log.Println("Error: Config path is empty, cannot open file.")
		ui.ShowNotification("Error Opening File", "Configuration file path is not set.")
		return
	}
	log.Printf("Request to open config file: %s", configPath)

	absPath, err := filepath.Abs(configPath)
	if err != nil {
		log.Printf("Warning: Failed to get absolute path for '%s': %v. Proceeding with original path.", configPath, err)
		absPath = configPath
	} else {
		log.Printf("Absolute config path resolved to: %s", absPath)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		log.Printf("Error: Config file does not exist at path: %s", absPath)
		ui.ShowNotification("Error Opening File", fmt.Sprintf("Config file not found: %s", absPath))
		return
	} else if err != nil {
		log.Printf("Error checking config file status at path '%s': %v", absPath, err)
	} else {
		log.Printf("Config file exists at: %s", absPath)
	}

	log.Printf("Attempting to open config file using ui.OpenFileInDefaultApp with path: %s", absPath)
	// Use the EXPORTED helper function from the ui package
	err = ui.OpenFileInDefaultApp(absPath) // <<< CORRECTED FUNCTION NAME
	if err != nil {
		log.Printf("Final error after trying open methods for config file: %v", err)
		ui.ShowNotification("Error Opening File", fmt.Sprintf("Could not open config file: %v", err))
	}
}