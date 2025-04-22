// ==== internal/ui/systray.go ====
package ui

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath" // <<< IMPORT ADDED/VERIFIED
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
	onOpenConfig   func()
	onViewLastDiff func() // Callback for viewing the diff
	embeddedIcon   []byte
	miRevert       *systray.MenuItem
	miViewLastDiff *systray.MenuItem // Menu item for viewing diff
	// Keep track of profile menu items to update checkmarks
	profileMenuItems map[int]*systray.MenuItem
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
	onOpenConfig func(),
	onViewLastDiff func(), // Add new parameter
) *SystrayManager {
	return &SystrayManager{
		config:           cfg,
		version:          version,
		embeddedIcon:     embeddedIcon,
		onReloadConfig:   onReloadConfig,
		onRestart:        onRestart,
		onQuit:           onQuit,
		onRevert:         onRevert,
		onOpenConfig:     onOpenConfig,
		onViewLastDiff:   onViewLastDiff, // Assign callback
		profileMenuItems: make(map[int]*systray.MenuItem),
	}
}

// UpdateConfig updates the configuration used by the systray manager
// and adjusts relevant UI elements. Rebuilds profile checkmarks.
func (s *SystrayManager) UpdateConfig(newCfg *config.Config) {
	log.Println("SystrayManager: Updating config reference.")
	s.config = newCfg

	// Update UI elements based on the new config

	// Update Revert menu item visibility/state based on the flag
	if s.miRevert != nil {
		if s.config.TemporaryClipboard {
			log.Println("SystrayManager: TemporaryClipboard is enabled in new config.")
			// State (enabled/disabled) is handled by UpdateRevertStatus based on actual clipboard state
		} else {
			log.Println("SystrayManager: TemporaryClipboard is disabled in new config. Disabling Revert menu item.")
			s.miRevert.Disable() // Permanently disable if feature is off
		}
	} else if s.config.TemporaryClipboard {
		// If the item didn't exist but should now, it requires a restart to add it.
		log.Println("SystrayManager: TemporaryClipboard is now enabled, but Revert item cannot be added without restart.")
	}


	// Update checkmarks on existing profile menu items
	// This assumes the menu items themselves persist across reloads without restart
	if s.profileMenuItems != nil {
		// Check existing items against potentially shorter new profile list
		for i, menuItem := range s.profileMenuItems {
			if menuItem == nil { continue } // Skip if somehow nil
			if i < len(s.config.Profiles) { // Check index bounds
				profile := s.config.Profiles[i]
				newText := "  " + profile.Name
				if profile.Enabled {
					newText = "✓ " + profile.Name
				}
				menuItem.SetTitle(newText)
				// Tooltip update could also happen here if needed
			} else {
                // If profile index is now out of bounds (profiles removed), hide/disable?
                // Systray doesn't easily support hiding. Disabling is an option.
                // menuItem.Disable()
				// Or just leave the potentially defunct menu item as is until restart. Let's leave it.
            }
		}
	}

	// Note: This does *not* rebuild the profile submenu structure itself
	// if profiles are added/removed. That still requires a restart.
}

// Run initializes and starts the system tray
func (s *SystrayManager) Run() {
	// Systray Run must be called on the main thread.
	systray.Run(s.onReady, s.onExit)
}

// UpdateRevertStatus enables or disables the revert menu item based on clipboard state
func (s *SystrayManager) UpdateRevertStatus(enabled bool) {
	if s.miRevert != nil {
		// Only allow enabling if the feature itself is enabled in the config
		if enabled && s.config.TemporaryClipboard {
			log.Println("SystrayManager: Enabling Revert menu item.")
			s.miRevert.Enable()
		} else {
			log.Println("SystrayManager: Disabling Revert menu item.")
			s.miRevert.Disable()
		}
	}
}


// UpdateViewLastDiffStatus enables or disables the view diff menu item
func (s *SystrayManager) UpdateViewLastDiffStatus(enabled bool) {
	if s.miViewLastDiff != nil {
		if enabled {
			log.Println("SystrayManager: Enabling View Last Change Details menu item.")
			// Use a consistent title, state is shown by enabled/disabled status
			s.miViewLastDiff.SetTitle("View Last Change Details")
			s.miViewLastDiff.Enable()
		} else {
			log.Println("SystrayManager: Disabling View Last Change Details menu item.")
			s.miViewLastDiff.SetTitle("View Last Change Details") // Keep title consistent
			s.miViewLastDiff.Disable()
		}
	}
}


// onReady is called by systray once the tray is ready.
func (s *SystrayManager) onReady() {
	// Set title and tooltip
	title := fmt.Sprintf("Clipboard Regex Replace %s", s.version)
	systray.SetTitle(title) // May not be visible on all platforms
	systray.SetTooltip(title)
	if len(s.embeddedIcon) > 0 {
		systray.SetIcon(s.embeddedIcon)
	} else {
		log.Println("Warning: No embedded icon data to set for systray.")
	}


	// Add version info (disabled)
	miVersion := systray.AddMenuItem(fmt.Sprintf("Version: %s", s.version), "Clipboard Regex Replace version")
	miVersion.Disable()
	systray.AddSeparator() // Separator after version

	// --- Dynamic Menu Structure based on Config ---
	// Build the profile submenu based on the *initial* config
	s.updateProfileMenuItems() // Creates the "Profiles" submenu and its initial items

	systray.AddSeparator() // Separator after profiles

	// Add configuration and application options
	miReloadConfig := systray.AddMenuItem("Reload Configuration", "Reload configuration from config.json")
	miOpenConfig := systray.AddMenuItem("Open Config File", "Open config.json in default editor")

	// *** Add View Last Change Details item ***
	s.miViewLastDiff = systray.AddMenuItem("View Last Change Details", "Show differences from the last replacement")
	s.miViewLastDiff.Disable() // Disabled initially

	miRestartApp := systray.AddMenuItem("Restart Application", "Completely restart the application to refresh menu")

	// Add clipboard revert option *only if* enabled in the initial config
	if s.config.TemporaryClipboard {
		log.Println("SystrayManager: TemporaryClipboard enabled, adding Revert menu item.")
		s.miRevert = systray.AddMenuItem("Revert to Original", "Revert to original clipboard text")
		s.miRevert.Disable() // Disabled initially until we have an original to revert to
	} else {
		log.Println("SystrayManager: TemporaryClipboard disabled, skipping Revert menu item creation.")
	}

	systray.AddSeparator() // Separator before quit

	// Add quit option
	miQuit := systray.AddMenuItem("Quit", "Exit the application")

	// --- Set up menu handlers in goroutines ---

	go func() {
		for range miReloadConfig.ClickedCh {
			log.Println("Reload Configuration menu item clicked.")
			if s.onReloadConfig != nil {
				s.onReloadConfig()
			}
		}
	}()

	go func() {
		for range miOpenConfig.ClickedCh {
			log.Println("Open Config File menu item clicked.")
			if s.onOpenConfig != nil {
				s.onOpenConfig()
			}
		}
	}()

	// Handler for View Last Change Details (check for nil item/callback)
	if s.miViewLastDiff != nil && s.onViewLastDiff != nil {
		go func() {
			for range s.miViewLastDiff.ClickedCh {
				log.Println("View Last Change Details menu item clicked.")
				s.onViewLastDiff() // Callback handles the logic
			}
		}()
	}

	go func() {
		for range miRestartApp.ClickedCh {
			log.Println("Restart Application menu item clicked.")
			if s.onRestart != nil {
				s.onRestart()
			}
		}
	}()

	// Handler for Revert (only if item exists and callback exists)
	if s.miRevert != nil && s.onRevert != nil {
		go func() {
			for range s.miRevert.ClickedCh {
				log.Println("Revert to Original menu item clicked.")
				s.onRevert()
			}
		}()
	}

	// Handler for Quit
	go func() {
		<-miQuit.ClickedCh
		log.Println("Quit menu item clicked.")
		if s.onQuit != nil {
			s.onQuit() // Allow app cleanup (like unregistering hotkeys)
		}
		systray.Quit() // Tell systray to exit
	}()

	log.Println("Systray ready and menu configured.")
}


// onExit is called when the systray is exiting
func (s *SystrayManager) onExit() {
	// Clean-up code here if needed
	log.Println("Systray exiting.")
}

// updateProfileMenuItems creates submenu items for each profile
// IMPORTANT: This builds the menu based on the config state *at the time it's called*.
// It doesn't dynamically update if profiles are added/removed via config reload without restart.
// It populates s.profileMenuItems for later checkmark updates.
func (s *SystrayManager) updateProfileMenuItems() {
	// Clear any previous items tracked (important if this were called repeatedly, though it's usually only at init)
	s.profileMenuItems = make(map[int]*systray.MenuItem)

	// Create a profiles submenu
	miProfiles := systray.AddMenuItem("Profiles", "Manage replacement profiles")

	// Add menu items for each profile found in the config
	if len(s.config.Profiles) > 0 {
		for i := range s.config.Profiles {
			// Capture loop variable correctly for closure
			profileIndex := i
			profile := &s.config.Profiles[profileIndex] // Get pointer for use inside goroutine

			// Create menu text with checkmark based on enabled status
			menuText := "  " + profile.Name // Default: no checkmark
			if profile.Enabled {
				menuText = "✓ " + profile.Name // Add checkmark if enabled
			}

			// Create menu item with tooltip showing hotkeys
			var tooltip string
			if profile.ReverseHotkey != "" {
				tooltip = fmt.Sprintf("Toggle profile: %s (Hotkey: %s, Reverse: %s)",
					profile.Name, profile.Hotkey, profile.ReverseHotkey)
			} else {
				tooltip = fmt.Sprintf("Toggle profile: %s (Hotkey: %s)",
					profile.Name, profile.Hotkey)
			}

			menuItem := miProfiles.AddSubMenuItem(menuText, tooltip)
			s.profileMenuItems[profileIndex] = menuItem // Store reference to the item

			// Handle clicks on the profile item - Toggle enable/disable
			go func(item *systray.MenuItem, idx int) {
				for range item.ClickedCh {
					// Access profile using the captured index 'idx' from the *current* config
					if idx >= len(s.config.Profiles) {
						log.Printf("Error: Profile index %d out of bounds after config change. Cannot toggle.", idx)
						ShowNotification("Menu Inconsistency", "Profile list changed. Please restart application.")
						continue // Avoid panic if profile was removed during reload
					}
					p := &s.config.Profiles[idx] // Get pointer to the profile in the current config

					// Toggle enabled status
					p.Enabled = !p.Enabled
					log.Printf("Toggled profile '%s' to enabled=%t", p.Name, p.Enabled)

					// Update menu item text immediately
					newText := "  " + p.Name
					if p.Enabled {
						newText = "✓ " + p.Name
					}
					item.SetTitle(newText)

					// --- Save config and reload ---
					if err := s.config.Save(); err != nil {
						log.Printf("Failed to save config after toggling profile '%s': %v", p.Name, err)
						ShowNotification("Save Error", fmt.Sprintf("Failed to save config after toggling '%s'", p.Name))
					} else {
						status := map[bool]string{true: "enabled", false: "disabled"}[p.Enabled]
						ShowNotification("Profile Updated",
							fmt.Sprintf("Profile '%s' has been %s.", p.Name, status))
						if s.onReloadConfig != nil {
							log.Println("Triggering internal config reload after profile toggle to update hotkeys.")
							time.Sleep(100 * time.Millisecond)
							s.onReloadConfig()
						}
					}
				}
			}(menuItem, profileIndex) // Pass menuItem and profileIndex to the goroutine
		} // End of profile loop

	} else {
		// If no profiles are defined in the config
		noProfilesItem := miProfiles.AddSubMenuItem("(No profiles defined)", "Add profiles in config.json")
		noProfilesItem.Disable()
	}

	// Add a separator workaround *within the submenu* before the "Add New Profile" option
	// miProfiles.AddSeparator() // <<< This caused the error
    // Workaround: Add a disabled item that looks like a separator
    sepItem := miProfiles.AddSubMenuItem("----------", "Separator") // Add to miProfiles submenu
    sepItem.Disable()                                              // Disable it


	// Add "Add New Profile" option *to the submenu*
	miAddProfile := miProfiles.AddSubMenuItem("➕ Add New Profile", "Adds a template profile to config.json (Restart Recommended)") // Add to miProfiles submenu

	// Handle clicks for adding a new profile
	go func() {
		for range miAddProfile.ClickedCh {
			log.Println("'Add New Profile' clicked.")
			newProfile := config.ProfileConfig{
				Name:          fmt.Sprintf("New Profile %s", time.Now().Format("150405")), // Compact time format
				Enabled:       true,
				Hotkey:        "ctrl+alt+n", // Suggest a default, user MUST edit this
				ReverseHotkey: "",           // Empty by default
				Replacements: []config.Replacement{
					{
						Regex:        "enter (?:your|a) regex pattern here",
						ReplaceWith:  "your replacement text",
						PreserveCase: false,
						ReverseWith:  "",
					},
				},
			}
			s.config.Profiles = append(s.config.Profiles, newProfile)
			if err := s.config.Save(); err != nil {
				log.Printf("Failed to save config after adding new profile template: %v", err)
				ShowNotification("Error Adding Profile", "Failed to save updated config file.")
				if len(s.config.Profiles) > 0 {
					s.config.Profiles = s.config.Profiles[:len(s.config.Profiles)-1]
				}
			} else {
				log.Printf("Added new profile template '%s' and saved config.", newProfile.Name)
				ShowNotification("Profile Template Added",
					fmt.Sprintf("Template '%s' added to config.json. Please edit it and use 'Restart Application' menu item.", newProfile.Name))
			}
		}
	}()
}


// isDevMode checks if the application is running in development mode via "go run"
func IsDevMode() bool {
	execPath, err := os.Executable()
	if err != nil {
		log.Printf("Warning: Could not get executable path in IsDevMode: %v", err)
		return false
	}
	// Check if the executable path contains typical temporary build directories
	isTempBuild := strings.Contains(execPath, "/go-build") || strings.Contains(execPath, "\\go-build") || strings.Contains(execPath, "/tmp/go-build") || strings.Contains(execPath, "\\Temp\\go-build")
	if isTempBuild {
		log.Printf("IsDevMode check: Detected temporary build path: %s. Assuming Dev Mode.", execPath)
		return true
	}
	// Fallback: Check if it's in the general temp directory
	tempDir := os.TempDir()
    // Use filepath.Clean to normalize paths before comparison
	if strings.HasPrefix(filepath.Clean(filepath.Dir(execPath)), filepath.Clean(tempDir)) { // <<< filepath used here
		log.Printf("IsDevMode check: Executable path (%s) is within Temp directory (%s). Assuming Dev Mode.", execPath, tempDir)
		return true
	}
	log.Printf("IsDevMode check: Executable='%s'. Assuming Production Mode.", execPath)
	return false
}

// RestartApplication attempts to restart the current application cleanly.
func RestartApplication() {
	log.Println("Attempting application restart...")

	// Check if we're running in development mode (go run)
	if IsDevMode() {
		log.Println("Development mode detected (running via 'go run' or from temp dir). Automatic restart is not supported.")
		ShowNotification("Manual Restart Needed", "App running in dev mode. Please stop (Ctrl+C) and run it again manually.")
		return // Prevent attempting to restart the temp binary
	}

	// Production mode - actually restart the compiled application executable
	execPath, err := os.Executable()
	if err != nil {
		log.Printf("Error getting executable path for restart: %v", err)
		ShowNotification("Restart Error", "Failed to get executable path.")
		return
	}

	cwd, err := os.Getwd()
	if err != nil {
		log.Printf("Warning: Could not get current working directory for restart: %v", err)
		cwd = ""
	}

	log.Printf("Attempting restart: Executable path: %s", execPath)
	if cwd != "" {
		log.Printf("Attempting restart: Setting working directory for new process: %s", cwd)
	} else {
		log.Printf("Attempting restart: Working directory not set for new process.")
	}

	cmd := exec.Command(execPath, os.Args[1:]...) // Pass original args
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if cwd != "" {
		cmd.Dir = cwd // Set the working directory for the new process
	}

	if err := cmd.Start(); err != nil {
		log.Printf("Error starting new process during restart: %v", err)
		ShowNotification("Restart Error", fmt.Sprintf("Failed to start new application process: %v", err))
		return
	}

	log.Println("Successfully started new process. Exiting current process now.")
	// Force exit of the old process. Assumes onQuit handled essential cleanup via the menu item handler.
	os.Exit(0)
}