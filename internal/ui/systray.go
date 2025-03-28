package ui

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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
) *SystrayManager {
	return &SystrayManager{
		config:         cfg,
		version:        version,
		onReloadConfig: onReloadConfig,
		onRestart:      onRestart,
		onQuit:         onQuit,
		onRevert:       onRevert,
		embeddedIcon:   embeddedIcon,
	}
}

// Run initializes and starts the system tray
func (s *SystrayManager) Run() {
	systray.Run(s.onReady, s.onExit)
}

// UpdateRevertStatus enables or disables the revert menu item
func (s *SystrayManager) UpdateRevertStatus(enabled bool) {
	if s.miRevert != nil {
		if enabled {
			s.miRevert.Enable()
		} else {
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

	// Add profiles menu
	s.updateProfileMenuItems()

	// Add configuration and application options
	miReloadConfig := systray.AddMenuItem("Reload Configuration", "Reload configuration from config.json")
	miRestartApp := systray.AddMenuItem("Restart Application", "Completely restart the application to refresh menu")

	// Add clipboard revert option if enabled
	if s.config.TemporaryClipboard {
		s.miRevert = systray.AddMenuItem("Revert to Original", "Revert to original clipboard text")
		s.miRevert.Disable() // Disabled initially until we have an original to revert to
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
		for range miRestartApp.ClickedCh {
			if s.onRestart != nil {
				s.onRestart()
			}
		}
	}()

	if s.config.TemporaryClipboard && s.miRevert != nil {
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
func (s *SystrayManager) updateProfileMenuItems() {
	// Create a profiles submenu
	miProfiles := systray.AddMenuItem("Profiles", "Manage replacement profiles")

	// Add menu items for each profile
	for i := range s.config.Profiles {
		profile := &s.config.Profiles[i]

		// Create menu text
		var menuText string
		if profile.Enabled {
			menuText = "✓ " + profile.Name
		} else {
			menuText = "  " + profile.Name
		}

		// Create menu item with tooltip
		var tooltip string
		if profile.ReverseHotkey != "" {
			tooltip = fmt.Sprintf("Toggle profile: %s (Hotkey: %s, Reverse: %s)",
				profile.Name, profile.Hotkey, profile.ReverseHotkey)
		} else {
			tooltip = fmt.Sprintf("Toggle profile: %s (Hotkey: %s)",
				profile.Name, profile.Hotkey)
		}

		menuItem := miProfiles.AddSubMenuItem(menuText, tooltip)

		// Handle clicks
		go func(p *config.ProfileConfig, item *systray.MenuItem) {
			for range item.ClickedCh {
				// Toggle enabled status
				p.Enabled = !p.Enabled

				// Update menu text
				if p.Enabled {
					item.SetTitle("✓ " + p.Name)
				} else {
					item.SetTitle("  " + p.Name)
				}

				// Save config
				if err := s.config.Save(); err != nil {
					log.Printf("Failed to save config after toggling profile: %v", err)
				}

				// Notify user
				status := map[bool]string{true: "enabled", false: "disabled"}[p.Enabled]
				ShowNotification("Profile Updated",
					fmt.Sprintf("Profile '%s' has been %s", p.Name, status))

				// Reload config is called to re-register hotkeys
				if s.onReloadConfig != nil {
					s.onReloadConfig()
				}
			}
		}(profile, menuItem)
	}

	// Add a separator
	miProfiles.AddSubMenuItem("----------", "")

	// Add new profile option
	miAddProfile := miProfiles.AddSubMenuItem("➕ Add New Profile", "Create a new replacement profile")

	// Handle add profile clicks
	go func() {
		for range miAddProfile.ClickedCh {
			// Create a new profile
			newProfile := config.ProfileConfig{
				Name:          fmt.Sprintf("New Profile %s", time.Now().Format("15:04:05")),
				Enabled:       true,
				Hotkey:        "ctrl+alt+n",
				ReverseHotkey: "", // Empty by default
				Replacements: []config.Replacement{
					{
						Regex:        "example",
						ReplaceWith:  "replacement",
						PreserveCase: false, // Default to false for backward compatibility
						ReverseWith:  "",    // Empty by default
					},
				},
			}

			// Add to config
			s.config.Profiles = append(s.config.Profiles, newProfile)

			// Save config
			if err := s.config.Save(); err != nil {
				log.Printf("Failed to save config after adding profile: %v", err)
			}

			// For adding profiles, we do need to restart to update the menu
			ShowNotification("Profile Added",
				fmt.Sprintf("New profile '%s' created. Restarting application to refresh menu.", newProfile.Name))

			// Wait a moment for notification to show before restarting
			time.Sleep(500 * time.Millisecond)
			
			// Call restart
			if s.onRestart != nil {
				s.onRestart()
			}
		}
	}()
}

// isDevMode checks if the application is running in development mode via "go run"
func IsDevMode() bool {
	execPath, err := os.Executable()
	if err != nil {
		return false
	}

	// Check if the executable is in a temporary directory, which indicates we're running via "go run"
	tempDir := os.TempDir()
	return strings.Contains(strings.ToLower(execPath), strings.ToLower(tempDir))
}

// RestartApplication restarts the current application
func RestartApplication() {
	log.Println("Restarting application...")

	// Check if we're running in development mode (go run)
	if IsDevMode() {
		log.Println("Development mode detected. Instead of restarting, refreshing UI components...")

		// In development mode, we won't actually restart
		// Just return and let the caller handle UI refresh
		ShowNotification("Dev Mode", "Menu changes will be visible after manually restarting the application")
		return
	}

	// Production mode - actually restart the application
	// Get the path to the current executable
	execPath, err := os.Executable()
	if err != nil {
		log.Printf("Error getting executable path: %v", err)
		ShowNotification("Error", "Failed to restart application")
		return
	}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Printf("Error getting current working directory: %v", err)
		ShowNotification("Error", "Failed to restart application")
		return
	}

	// Log paths for debugging
	log.Printf("Executable path: %s", execPath)
	log.Printf("Current working directory: %s", cwd)
	log.Printf("Config should be at: %s", filepath.Join(cwd, "config.json"))

	// Check if config file exists
	if _, err := os.Stat(filepath.Join(cwd, "config.json")); err != nil {
		log.Printf("Warning: Config file check failed: %v", err)
	} else {
		log.Printf("Config file exists and is accessible")
	}

	// Start a new process with the same executable
	cmd := exec.Command(execPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = cwd // Set the working directory to the current directory

	// Start the new process
	if err := cmd.Start(); err != nil {
		log.Printf("Error starting new process: %v", err)
		ShowNotification("Error", "Failed to restart application")
		return
	}

	// Exit the current process
	os.Exit(0)
}