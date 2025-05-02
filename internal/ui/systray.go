// ==== internal/ui/systray.go ====
package ui

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	//"regexp" // <-- Import regexp
	"strings"
	"time"

	"github.com/getlantern/systray"
	"github.com/TanaroSch/clipboard-regex-replace/internal/config"
	// *** Ensure golang.org/x/term is NOT imported if no longer used ***
)

// SystrayManager handles the system tray icon and menu
type SystrayManager struct {
	config           *config.Config
	version          string
	onReloadConfig   func()
	onRestart        func()
	onQuit           func()
	onRevert         func()
	onOpenConfig     func()
	onViewLastDiff   func()
	onAddSecret      func() // Callback for Add/Update Secret
	onListSecrets    func() // Callback for List Secrets
	onRemoveSecret   func() // Callback for Remove Secret
	onAddSimpleRule  func() // <-- Add callback for simple rule
	embeddedIcon     []byte
	miRevert         *systray.MenuItem
	miViewLastDiff   *systray.MenuItem
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
	onViewLastDiff func(),
	onAddSecret func(),
	onListSecrets func(),
	onRemoveSecret func(),
	onAddSimpleRule func(), // <-- Add parameter for simple rule callback
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
		onViewLastDiff:   onViewLastDiff,
		profileMenuItems: make(map[int]*systray.MenuItem),
		onAddSecret:      onAddSecret,
		onListSecrets:    onListSecrets,
		onRemoveSecret:   onRemoveSecret,
		onAddSimpleRule:  onAddSimpleRule, // <-- Store the callback
	}
}

// UpdateConfig updates the configuration used by the systray manager
// and adjusts relevant UI elements like Revert status and Profile checkmarks.
func (s *SystrayManager) UpdateConfig(newCfg *config.Config) {
	log.Println("SystrayManager: Updating config reference.")
	s.config = newCfg // Update internal reference

	// Update Revert menu item state based on config flag
	if s.miRevert != nil {
		if s.config != nil && s.config.TemporaryClipboard {
			log.Println("SystrayManager: TemporaryClipboard is enabled in new config.")
			// Enable/disable state is handled by UpdateRevertStatus based on clipboard content
		} else {
			log.Println("SystrayManager: TemporaryClipboard disabled or config nil. Disabling Revert menu item.")
			s.miRevert.Disable() // Permanently disable if feature is off
		}
	} else if s.config != nil && s.config.TemporaryClipboard {
		// If the item didn't exist but should now, it requires a restart to add it.
		log.Println("SystrayManager: TemporaryClipboard is now enabled, but Revert item cannot be added without restart.")
	}

	// Update checkmarks on existing profile menu items
	if s.profileMenuItems != nil && s.config != nil && s.config.Profiles != nil {
		log.Printf("SystrayManager: Updating profile menu item checkmarks (%d items, %d profiles)", len(s.profileMenuItems), len(s.config.Profiles))
		for i, menuItem := range s.profileMenuItems {
			if menuItem == nil {
				log.Printf("SystrayManager: Skipping nil profile menu item at index %d", i)
				continue
			}
			if i < len(s.config.Profiles) { // Check index bounds against NEW config
				profile := s.config.Profiles[i] // Get profile from NEW config
				log.Printf("SystrayManager: Updating checkmark for profile '%s' (index %d, enabled: %t)", profile.Name, i, profile.Enabled)
				newText := "  " + profile.Name
				if profile.Enabled {
					newText = "✓ " + profile.Name
				}
				// Update Title AND Checked status for clarity if API supports it (getlantern/systray typically uses title prefix)
				menuItem.SetTitle(newText)
				// menuItem.SetChecked(profile.Enabled) // Use if SetChecked is available and preferred

			} else {
				log.Printf("SystrayManager: Profile index %d is out of bounds after reload. Disabling related menu item.", i)
				menuItem.Disable()
			}
		}
	} else {
		log.Println("SystrayManager: Skipping profile menu item update (no items or no profiles in config).")
	}
}

// Run initializes and starts the system tray
func (s *SystrayManager) Run() {
	systray.Run(s.onReady, s.onExit)
}

// UpdateRevertStatus enables or disables the revert menu item based on clipboard state
func (s *SystrayManager) UpdateRevertStatus(enabled bool) {
	if s.miRevert != nil {
		// Only allow enabling if the feature itself is enabled in the config
		if enabled && s.config != nil && s.config.TemporaryClipboard {
			log.Println("SystrayManager: Enabling Revert menu item.")
			s.miRevert.Enable()
		} else {
			// Disable if not enabled OR if the feature is turned off
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
			s.miViewLastDiff.SetTitle("View Last Change Details") // Ensure title is correct
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
	systray.SetTitle(title)
	systray.SetTooltip(title)
	if len(s.embeddedIcon) > 0 {
		systray.SetIcon(s.embeddedIcon)
	} else {
		log.Println("Warning: No embedded icon data to set for systray.")
	}

	// Add version info (disabled)
	miVersion := systray.AddMenuItem(fmt.Sprintf("Version: %s", s.version), "Clipboard Regex Replace version")
	miVersion.Disable()
	systray.AddSeparator()

	// Build the profile submenu
	s.updateProfileMenuItems() // This already has "Add New Profile"
	systray.AddSeparator()

	// --- Add Secret Management Menu ---
	miManageSecrets := systray.AddMenuItem("Manage Secrets", "Add/Remove sensitive values")
	miAddSecret := miManageSecrets.AddSubMenuItem("Add/Update Secret...", "Store a new sensitive value")
	miListSecrets := miManageSecrets.AddSubMenuItem("List Secret Names", "Show names of stored secrets")
	miRemoveSecret := miManageSecrets.AddSubMenuItem("Remove Secret...", "Delete a stored secret")

	// --- Add Simple Rule Menu Item ---
	miAddSimpleRule := systray.AddMenuItem("Add Simple Rule...", "Add a 1:1 text replacement rule to a profile") // <-- New Item

	systray.AddSeparator()

	// Config & App Control - Update tooltips for restart requirement
	miReloadConfig := systray.AddMenuItem("Reload Configuration", "Reload config (manual restart needed for new secrets/hotkeys)")
	miOpenConfig := systray.AddMenuItem("Open Config File", "Open config.json in default editor")
	s.miViewLastDiff = systray.AddMenuItem("View Last Change Details", "Show differences from the last replacement")
	s.miViewLastDiff.Disable()
	miRestartApp := systray.AddMenuItem("Restart Application", "Restart (needed after adding/removing secrets or profiles)")

	// Revert Option
	if s.config != nil && s.config.TemporaryClipboard {
		log.Println("SystrayManager: TemporaryClipboard enabled, adding Revert menu item.")
		s.miRevert = systray.AddMenuItem("Revert to Original", "Revert to original clipboard text")
		s.miRevert.Disable()
	} else {
		log.Println("SystrayManager: TemporaryClipboard disabled or config nil, skipping Revert menu item creation.")
	}

	systray.AddSeparator()
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
	if s.miViewLastDiff != nil && s.onViewLastDiff != nil {
		go func() {
			for range s.miViewLastDiff.ClickedCh {
				log.Println("View Last Change Details menu item clicked.")
				s.onViewLastDiff()
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
	if s.miRevert != nil && s.onRevert != nil {
		go func() {
			for range s.miRevert.ClickedCh {
				log.Println("Revert to Original menu item clicked.")
				s.onRevert()
			}
		}()
	}

	// Secret Handlers
	if s.onAddSecret != nil {
		go func() {
			for range miAddSecret.ClickedCh {
				log.Println("Add/Update Secret menu item triggered.")
				s.onAddSecret()
			}
		}()
	}
	if s.onListSecrets != nil {
		go func() {
			for range miListSecrets.ClickedCh {
				log.Println("List Secret Names menu item triggered.")
				s.onListSecrets()
			}
		}()
	}
	if s.onRemoveSecret != nil {
		go func() {
			for range miRemoveSecret.ClickedCh {
				log.Println("Remove Secret menu item triggered.")
				s.onRemoveSecret()
			}
		}()
	}

	// Add Simple Rule Handler <-- New Handler
	if s.onAddSimpleRule != nil {
		go func() {
			for range miAddSimpleRule.ClickedCh {
				log.Println("'Add Simple Rule...' menu item triggered.")
				s.onAddSimpleRule()
			}
		}()
	}

	// Quit Handler
	go func() {
		<-miQuit.ClickedCh
		log.Println("Quit menu item clicked.")
		if s.onQuit != nil {
			s.onQuit()
		}
		systray.Quit()
	}()

	log.Println("Systray ready and menu configured.")
}

// onExit is called when the systray is exiting
func (s *SystrayManager) onExit() {
	log.Println("Systray exiting.")
}

// updateProfileMenuItems creates submenu items for each profile
func (s *SystrayManager) updateProfileMenuItems() {
	s.profileMenuItems = make(map[int]*systray.MenuItem)
	miProfiles := systray.AddMenuItem("Profiles", "Manage replacement profiles")
	if s.config != nil && len(s.config.Profiles) > 0 {
		for i := range s.config.Profiles {
			profileIndex := i // Capture index for goroutine closure
			if profileIndex >= len(s.config.Profiles) {
				continue // Safety check in case config changes during loop setup
			}
			profile := s.config.Profiles[profileIndex]
			menuText := "  " + profile.Name
			if profile.Enabled {
				menuText = "✓ " + profile.Name
			}
			var tooltip string
			if profile.ReverseHotkey != "" {
				tooltip = fmt.Sprintf("Toggle profile: %s (Hotkey: %s, Reverse: %s)", profile.Name, profile.Hotkey, profile.ReverseHotkey)
			} else {
				tooltip = fmt.Sprintf("Toggle profile: %s (Hotkey: %s)", profile.Name, profile.Hotkey)
			}
			menuItem := miProfiles.AddSubMenuItem(menuText, tooltip)
			s.profileMenuItems[profileIndex] = menuItem

			go func(item *systray.MenuItem, idx int) {
				for range item.ClickedCh {
					// --- Safely access config and profile ---
					if s.config == nil || s.config.Profiles == nil || idx >= len(s.config.Profiles) {
						errMsg := "Profile list changed unexpectedly. Please use Reload or Restart."
						log.Printf("Error: Profile index %d out of bounds or config nil after config change. Cannot toggle.", idx)
						ShowAdminNotification(LevelWarn, "Menu Inconsistency", errMsg) // <<< CHANGED (Warn Level)
						continue
					}
					p := &s.config.Profiles[idx] // Get pointer to modify directly

					// --- Toggle State ---
					p.Enabled = !p.Enabled
					log.Printf("Toggled profile '%s' to enabled=%t", p.Name, p.Enabled)

					// --- Update Menu Item Visual ---
					newText := "  " + p.Name
					if p.Enabled {
						newText = "✓ " + p.Name
					}
					item.SetTitle(newText)

					// --- Save Config ---
					if err := s.config.Save(); err != nil {
						errMsg := fmt.Sprintf("Failed to save config after toggling '%s'. Error: %v", p.Name, err)
						log.Printf("Failed to save config after toggling profile '%s': %v", p.Name, err)
						ShowAdminNotification(LevelError, "Save Error", errMsg) // <<< CHANGED (Error Level)
						// Optionally attempt to revert in-memory state
						p.Enabled = !p.Enabled
						newText = "  " + p.Name
						if p.Enabled { newText = "✓ " + p.Name }
						item.SetTitle(newText)
					} else {
						// --- Notify & Reload ---
						status := map[bool]string{true: "enabled", false: "disabled"}[p.Enabled]
						msg := fmt.Sprintf("Profile '%s' has been %s. Reloading...", p.Name, status)
						ShowAdminNotification(LevelInfo, "Profile Updated", msg) // <<< CHANGED (Info Level)
						if s.onReloadConfig != nil {
							log.Println("Triggering internal config reload after profile toggle to update hotkeys.")
							// Slight delay to allow notification to potentially show first
							time.Sleep(150 * time.Millisecond)
							s.onReloadConfig()
						}
					}
				}
			}(menuItem, profileIndex)
		}
	} else {
		log.Println("No profiles defined in config or config is nil.")
		noProfilesItem := miProfiles.AddSubMenuItem("(No profiles defined)", "Add profiles in config.json")
		noProfilesItem.Disable()
	}
	sepItem := miProfiles.AddSubMenuItem("----------", "Separator")
	sepItem.Disable()
	miAddProfile := miProfiles.AddSubMenuItem("➕ Add New Profile", "Adds a template profile to config.json (Restart Recommended)")
	go func() {
		for range miAddProfile.ClickedCh {
			log.Println("'Add New Profile' clicked.")
			if s.config == nil {
				log.Println("Error: Cannot add profile, config is nil.")
				ShowAdminNotification(LevelError, "Internal Error", "Application configuration not loaded.") // <<< CHANGED (Error Level)
				continue
			}
			// Ensure unique name generation remains robust even with frequent additions
			baseName := "New_Profile"
			existingNames := make(map[string]bool)
			if s.config.Profiles != nil {
				for _, p := range s.config.Profiles {
					existingNames[p.Name] = true
				}
			}

			newProfileName := baseName
			counter := 1
			for existingNames[newProfileName] {
				newProfileName = fmt.Sprintf("%s_%d", baseName, counter)
				counter++
			}

			newProfile := config.ProfileConfig{
				Name:    newProfileName,
				Enabled: true,
				Hotkey:  "ctrl+alt+n", // Default new hotkey, might need adjustment by user
				Replacements: []config.Replacement{
					{
						Regex:       fmt.Sprintf("text_for_%s", newProfileName),
						ReplaceWith: "replacement_text",
					},
				},
			}
			s.config.Profiles = append(s.config.Profiles, newProfile)
			if err := s.config.Save(); err != nil {
				errMsg := fmt.Sprintf("Failed to save updated config file. Error: %v", err)
				log.Printf("Failed to save config after adding new profile template: %v", err)
				ShowAdminNotification(LevelError, "Error Adding Profile", errMsg) // <<< CHANGED (Error Level)
				// Roll back in-memory change on save failure
				if len(s.config.Profiles) > 0 {
					s.config.Profiles = s.config.Profiles[:len(s.config.Profiles)-1]
				}
			} else {
				msg := fmt.Sprintf("Template '%s' added. Edit config.json and use 'Reload' or 'Restart Application'.", newProfile.Name)
				log.Printf("Added new profile template '%s' and saved config.", newProfile.Name)
				ShowAdminNotification(LevelInfo, "Profile Template Added", msg) // <<< CHANGED (Info Level)
				// Note: The menu won't update automatically without a restart or explicit refresh logic
			}
		}
	}()
}

// IsDevMode checks if the application is running in development mode
func IsDevMode() bool {
	execPath, err := os.Executable()
	if err != nil {
		log.Printf("Warning: Could not get executable path in IsDevMode: %v", err)
		return false // Assume not dev mode on error
	}
	// Check if the executable path contains typical temporary build directories
	isTempBuild := strings.Contains(execPath, string(filepath.Separator)+"go-build") ||
		strings.Contains(execPath, string(filepath.Separator)+"tmp"+string(filepath.Separator)+"go-build") || // Linux /tmp
		strings.Contains(execPath, string(filepath.Separator)+"Temp"+string(filepath.Separator)+"go-build") // Windows %TEMP%
	if isTempBuild {
		log.Printf("IsDevMode check: Detected temporary build path: %s. Assuming Dev Mode.", execPath)
		return true
	}
	// Fallback: Check if it's in the general temp directory
	tempDir := os.TempDir()
	// Ensure paths are clean and compare directory prefixes
	cleanedExecDir := filepath.Clean(filepath.Dir(execPath))
	cleanedTempDir := filepath.Clean(tempDir)
	if strings.HasPrefix(cleanedExecDir, cleanedTempDir) {
		log.Printf("IsDevMode check: Executable path (%s) is within Temp directory (%s). Assuming Dev Mode.", cleanedExecDir, cleanedTempDir)
		return true
	}
	log.Printf("IsDevMode check: Executable='%s'. Assuming Production Mode.", execPath)
	return false
}

// RestartApplication attempts to restart the current application cleanly.
func RestartApplication() {
	log.Println("Attempting application restart...")
	if IsDevMode() {
		msg := "App running in dev mode. Please stop and run it again manually."
		log.Println("Development mode detected. Automatic restart is not supported.")
		ShowAdminNotification(LevelWarn, "Manual Restart Needed", msg) // <<< CHANGED (Warn Level)
		return
	}
	execPath, err := os.Executable()
	if err != nil {
		errMsg := fmt.Sprintf("Failed to get executable path. Error: %v", err)
		log.Printf("Error getting executable path for restart: %v", err)
		ShowAdminNotification(LevelError, "Restart Error", errMsg) // <<< CHANGED (Error Level)
		return
	}
	cwd, err := os.Getwd()
	if err != nil {
		log.Printf("Warning: Could not get CWD for restart: %v.", err)
		cwd = ""
	}
	log.Printf("Attempting restart: Executable path: %s", execPath)
	if cwd != "" {
		log.Printf("Attempting restart: Setting CWD: %s", cwd)
	} else {
		log.Printf("Attempting restart: CWD not set.")
	}
	cmd := exec.Command(execPath, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if cwd != "" {
		cmd.Dir = cwd
	}
	if err := cmd.Start(); err != nil {
		errMsg := fmt.Sprintf("Failed to start new application process: %v", err)
		log.Printf("Error starting new process during restart: %v", err)
		ShowAdminNotification(LevelError, "Restart Error", errMsg) // <<< CHANGED (Error Level)
		return
	}
	log.Println("Successfully started new process. Exiting current process now.")
	systray.Quit() // Use systray.Quit() to try and trigger onExit cleanly
	os.Exit(0)     // Fallback exit
}

// GetAppIcon retrieves the embedded icon data.
// Consider moving resource loading logic here or making it globally accessible if needed elsewhere in ui.
func GetAppIcon() ([]byte, error) {
	// This is a placeholder implementation. You should use your existing resource loading.
	// Example using the pattern from internal/resources:
	// return resources.GetIcon()
	return nil, fmt.Errorf("icon loading not implemented in ui.GetAppIcon")
}