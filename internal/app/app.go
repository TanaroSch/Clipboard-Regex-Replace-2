package app

import (
	"fmt"
	"log"

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
	newConfig, err := config.Load(a.config.GetConfigPath())
	if err != nil {
		log.Printf("Error reloading configuration: %v", err)
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
		for _, profile := range a.config.Profiles {
			if !originalProfileNames[profile.Name] {
				profileStructureChanged = true
				break
			}
		}
	}

	log.Println("Configuration reloaded successfully.")

	// Re-register hotkeys
	a.hotkeyManager.RegisterAll()

	if profileStructureChanged {
		// If profile structure changed, we need to restart to rebuild the menu
		ui.ShowNotification("Configuration Reloaded",
			"Profile structure has changed. Restarting application to refresh menu.")

		// Wait a moment for notification to show
		a.onRestartApplication()
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