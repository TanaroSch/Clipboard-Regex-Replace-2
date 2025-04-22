// ==== internal/ui/systray.go ====
package ui

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	// "path/filepath" // Removed unused import
	"strings"
	"time"

	"github.com/getlantern/systray"
	"github.com/TanaroSch/clipboard-regex-replace/internal/config"
)

// SystrayManager handles the system tray icon and menu
type SystrayManager struct {
	config         *config.Config
	version        string
	onReloadConfig func()
	onRestart      func()
	onQuit         func()
	onRevert       func()
	onOpenConfig   func() // Callback to open config file
	embeddedIcon   []byte
	miRevert       *systray.MenuItem
}

// NewSystrayManager creates a new system tray manager
func NewSystrayManager(
	cfg *config.Config,
	version string,
	embeddedIcon []byte,
	onReloadConfig func(),
	onRestart func(),
	onQuit func(),
	onRevert func(),
	onOpenConfig func(), // New parameter
) *SystrayManager {
	return &SystrayManager{
		config:         cfg,
		version:        version,
		onReloadConfig: onReloadConfig,
		onRestart:      onRestart,
		onQuit:         onQuit,
		onRevert:       onRevert,
		onOpenConfig:   onOpenConfig, // Assign new callback
		embeddedIcon:   embeddedIcon,
	}
}

// UpdateConfig updates the configuration used by the systray manager
// and adjusts relevant UI elements. Note that this does *not* rebuild
// the profile submenu, as that often requires an application restart.
func (s *SystrayManager) UpdateConfig(newCfg *config.Config) {
	log.Println("SystrayManager: Updating config reference.")
	s.config = newCfg

	// Update UI elements based on the new config
	// Example: Re-evaluate the Revert menu item visibility/state
	if s.miRevert != nil {
		if s.config.TemporaryClipboard {
			log.Println("SystrayManager: TemporaryClipboard is enabled in new config, ensuring Revert menu item exists (status unchanged for now).")
			// The item already exists if miRevert is not nil.
			// Its enabled/disabled status is handled separately by UpdateRevertStatus based on clipboard state,
			// not directly by the config flag after initial creation.
		} else {
			log.Println("SystrayManager: TemporaryClipboard is disabled in new config. Disabling Revert menu item permanently.")
			// Note: getlantern/systray doesn't directly support removing/hiding an existing item easily.
			// Disabling it is the practical approach here. If the original state allows reverting,
			// UpdateRevertStatus(false) might be called later, but this ensures it stays disabled if the feature is off.
			s.miRevert.Disable()
			// A more complex implementation might try to fully remove/re-add, but restart is safer.
		}
	}
	// Other UI updates based on config could go here if needed.
}

// Run initializes and starts the system tray
func (s *SystrayManager) Run() {
	systray.Run(s.onReady, s.onExit)
}

// UpdateRevertStatus enables or disables the revert menu item
func (s *SystrayManager) UpdateRevertStatus(enabled bool) {
	if s.miRevert != nil {
		// Only allow enabling if the feature is enabled in the config
		if enabled && s.config.TemporaryClipboard {
			log.Println("SystrayManager: Enabling Revert menu item.")
			s.miRevert.Enable()
		} else {
			log.Println("SystrayManager: Disabling Revert menu item.")
			s.miRevert.Disable()
		}
	}
}

// onReady is called by systray once the tray is ready.
func (s *SystrayManager) onReady() {
	// Set title and tooltip
	systray.SetTitle(fmt.Sprintf("Clipboard Regex Replace %s", s.version))
	systray.SetTooltip(fmt.Sprintf("Clipboard Regex Replace %s", s.version))
	systray.SetIcon(s.embeddedIcon)

	// Add version info (disabled)
	miVersion := systray.AddMenuItem(fmt.Sprintf("Version: %s", s.version), "Clipboard Regex Replace version")
	miVersion.Disable()

	// --- Dynamic Menu Structure based on Config ---
	// It's better to build the menu dynamically here based *initial* config
	// as dynamically adding/removing top-level items later is tricky.

	// Add profiles menu
	s.updateProfileMenuItems() // This reads s.config internally

	// Add configuration and application options
	miReloadConfig := systray.AddMenuItem("Reload Configuration", "Reload configuration from config.json")
	miOpenConfig := systray.AddMenuItem("Open Config File", "Open config.json in default editor")
	miRestartApp := systray.AddMenuItem("Restart Application", "Completely restart the application to refresh menu")

	// Add clipboard revert option *only if* enabled in the initial config
	if s.config.TemporaryClipboard {
		log.Println("SystrayManager: TemporaryClipboard enabled, adding Revert menu item.")
		s.miRevert = systray.AddMenuItem("Revert to Original", "Revert to original clipboard text")
		s.miRevert.Disable() // Disabled initially until we have an original to revert to
	} else {
		log.Println("SystrayManager: TemporaryClipboard disabled, skipping Revert menu item creation.")
	}

	// Add quit option
	miQuit := systray.AddMenuItem("Quit", "Exit the application")

	// Set up menu handlers
	go func() {
		for range miReloadConfig.ClickedCh {
			if s.onReloadConfig != nil {
				s.onReloadConfig()
			}
		}
	}()

	go func() {
		for range miOpenConfig.ClickedCh {
			if s.onOpenConfig != nil {
				log.Println("Open Config File menu item clicked.")
				s.onOpenConfig()
			}
		}
	}()

	go func() {
		for range miRestartApp.ClickedCh {
			if s.onRestart != nil {
				s.onRestart()
			}
		}
	}()

	// Only create the channel listener if the menu item was created
	if s.miRevert != nil {
		go func() {
			for range s.miRevert.ClickedCh {
				if s.onRevert != nil {
					s.onRevert()
				}
			}
		}()
	}

	go func() {
		<-miQuit.ClickedCh
		if s.onQuit != nil {
			s.onQuit()
		}
		systray.Quit()
		log.Println("Exiting application.")
	}()
}

// onExit is called when the systray is exiting
func (s *SystrayManager) onExit() {
	// Clean-up code here if needed
}

// updateProfileMenuItems creates submenu items for each profile
// IMPORTANT: This builds the menu based on the config state *at the time it's called*.
// It doesn't dynamically update if profiles are added/removed via config reload without restart.
func (s *SystrayManager) updateProfileMenuItems() {
	// Create a profiles submenu
	miProfiles := systray.AddMenuItem("Profiles", "Manage replacement profiles")

	// Add menu items for each profile
	if len(s.config.Profiles) > 0 {
		for i := range s.config.Profiles {
			// Capture loop variable correctly for closure
			profileIndex := i

			// Create menu text
			menuText := "  " + s.config.Profiles[profileIndex].Name
			if s.config.Profiles[profileIndex].Enabled {
				menuText = "✓ " + s.config.Profiles[profileIndex].Name
			}

			// Create menu item with tooltip
			var tooltip string
			profile := &s.config.Profiles[profileIndex] // Get pointer for use in goroutine
			if profile.ReverseHotkey != "" {
				tooltip = fmt.Sprintf("Toggle profile: %s (Hotkey: %s, Reverse: %s)",
					profile.Name, profile.Hotkey, profile.ReverseHotkey)
			} else {
				tooltip = fmt.Sprintf("Toggle profile: %s (Hotkey: %s)",
					profile.Name, profile.Hotkey)
			}

			menuItem := miProfiles.AddSubMenuItem(menuText, tooltip)

			// Handle clicks - Toggle profile enable/disable
			go func(item *systray.MenuItem) {
				for range item.ClickedCh {
					// Access profile via index from the *current* config, in case it was updated
					if profileIndex >= len(s.config.Profiles) {
						log.Printf("Error: Profile index %d out of bounds after config change.", profileIndex)
						continue // Avoid panic if profile was removed during reload
					}
					p := &s.config.Profiles[profileIndex]

					// Toggle enabled status
					p.Enabled = !p.Enabled
					log.Printf("Toggled profile '%s' to enabled=%t", p.Name, p.Enabled)

					// Update menu text
					newText := "  " + p.Name
					if p.Enabled {
						newText = "✓ " + p.Name
					}
					item.SetTitle(newText)

					// Save config
					if err := s.config.Save(); err != nil {
						log.Printf("Failed to save config after toggling profile: %v", err)
						// Optionally notify user of save error
					}

					// Notify user
					status := map[bool]string{true: "enabled", false: "disabled"}[p.Enabled]
					ShowNotification("Profile Updated",
						fmt.Sprintf("Profile '%s' has been %s", p.Name, status))

					// Reload config internally to re-register hotkeys based on new enabled state
					// This avoids needing a full app restart just for toggling.
					if s.onReloadConfig != nil {
						log.Println("Triggering internal config reload after profile toggle to update hotkeys.")
						s.onReloadConfig()
					}
				}
			}(menuItem) // Pass menuItem to the goroutine
		}
	} else {
		noProfilesItem := miProfiles.AddSubMenuItem("(No profiles defined)", "Add profiles in config.json")
		noProfilesItem.Disable()
	}

	// Add a separator
	// miProfiles.AddSeparator() // If this line causes compilation errors consistently...
	// Workaround: Add a disabled item that looks like a separator
	sepItem := miProfiles.AddSubMenuItem("----------", "Separator")
	sepItem.Disable()

	// Add new profile option
	miAddProfile := miProfiles.AddSubMenuItem("➕ Add New Profile", "Create a new replacement profile (Requires Restart)")

	// Handle add profile clicks
	go func() {
		for range miAddProfile.ClickedCh {
			// Create a new profile structure
			newProfile := config.ProfileConfig{
				Name:          fmt.Sprintf("New Profile %s", time.Now().Format("150405")), // Compact time format
				Enabled:       true,
				Hotkey:        "ctrl+alt+n", // Suggest a default, user must edit
				ReverseHotkey: "",           // Empty by default
				Replacements: []config.Replacement{
					{
						Regex:        "example regex",
						ReplaceWith:  "example replacement",
						PreserveCase: false,
						ReverseWith:  "",
					},
				},
			}

			// Add to config in memory
			s.config.Profiles = append(s.config.Profiles, newProfile)

			// Save config to file
			if err := s.config.Save(); err != nil {
				log.Printf("Failed to save config after adding profile: %v", err)
				ShowNotification("Error", "Failed to save updated config file.")
				// Decide not to restart if save failed? Or proceed anyway?
			} else {
				log.Printf("Added new profile '%s' and saved config.", newProfile.Name)
				// Notify user that restart is needed because menu won't update automatically
				ShowNotification("Profile Added",
					fmt.Sprintf("New profile '%s' added to config.json. Please use 'Restart Application' menu item to see it in the list.", newProfile.Name))
			}

			// We do NOT call restart automatically here.
			// Let the user trigger restart via the dedicated menu item after they've potentially
			// edited the new profile's details in the config file.
		}
	}()
}

// isDevMode checks if the application is running in development mode via "go run"
func IsDevMode() bool {
	execPath, err := os.Executable()
	if err != nil {
		// Default to false if executable path cannot be determined
		log.Printf("Warning: Could not get executable path in IsDevMode: %v", err)
		return false
	}

	// Check if the executable is in a temporary directory, common for "go run"
	tempDir := os.TempDir()
	isTemp := strings.Contains(strings.ToLower(execPath), strings.ToLower(tempDir))
	log.Printf("IsDevMode check: Executable='%s', TempDir='%s', IsTemp=%t", execPath, tempDir, isTemp)
	return isTemp

	// Alternative check: Look for specific patterns if the temp dir check is unreliable
	// return strings.Contains(execPath, "/go-build") || strings.Contains(execPath, "\\go-build") || strings.HasPrefix(filepath.Base(execPath), "main")
}

// RestartApplication restarts the current application
func RestartApplication() {
	log.Println("Restarting application...")

	// Check if we're running in development mode (go run)
	if IsDevMode() {
		log.Println("Development mode detected. Manual restart required.")
		// In development mode, actually restarting the 'go run' process is complex and often unwanted.
		ShowNotification("Restart Needed", "App is running via 'go run'. Please stop (Ctrl+C) and run it again manually.")
		return
	}

	// Production mode - actually restart the application
	// Get the path to the current executable
	execPath, err := os.Executable()
	if err != nil {
		log.Printf("Error getting executable path: %v", err)
		ShowNotification("Restart Error", "Failed to get executable path.")
		return
	}

	// Get current working directory to preserve it for the new process
	cwd, err := os.Getwd()
	if err != nil {
		log.Printf("Error getting current working directory: %v", err)
		// Try to proceed without setting CWD, might work if config is relative to exe
		cwd = "" // Or maybe return an error? For now, try proceeding.
		ShowNotification("Restart Warning", "Could not get working directory, restart might fail.")
		// return // Option: prevent restart if CWD fails
	}

	// Log paths for debugging
	log.Printf("Attempting restart: Executable path: %s", execPath)
	if cwd != "" {
		log.Printf("Attempting restart: Setting working directory: %s", cwd)
	}

	// Prepare the command to run the executable again
	cmd := exec.Command(execPath)
	cmd.Stdout = os.Stdout // Redirect stdout/stderr if needed for debugging the new instance
	cmd.Stderr = os.Stderr
	if cwd != "" {
		cmd.Dir = cwd // Set the working directory
	}

	// Start the new process without waiting for it
	if err := cmd.Start(); err != nil {
		log.Printf("Error starting new process: %v", err)
		ShowNotification("Restart Error", "Failed to start new application process.")
		return
	}

	log.Println("Successfully started new process. Exiting current process.")
	// Exit the current process cleanly
	// Use systray.Quit() for graceful shutdown if possible, otherwise os.Exit(0)
	// systray.Quit() // This might be called already by the Quit menu handler context
	os.Exit(0) // Force exit if systray.Quit() isn't appropriate here
}