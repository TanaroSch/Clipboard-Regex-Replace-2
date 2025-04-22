// ==== internal/app/app.go ====
package app

import (
	"errors" // Needed for zenity error checking
	"fmt"    // Needed for Sprintf
	"log"
	"os" // Needed by os.Exit in RestartApplication call chain
	"path/filepath"
	"strings"

	"github.com/TanaroSch/clipboard-regex-replace/internal/clipboard"
	"github.com/TanaroSch/clipboard-regex-replace/internal/config"
	"github.com/TanaroSch/clipboard-regex-replace/internal/hotkey"
	"github.com/TanaroSch/clipboard-regex-replace/internal/resources"
	"github.com/TanaroSch/clipboard-regex-replace/internal/ui"
	"github.com/ncruces/zenity" // Zenity import
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
		config:  cfg, // Config now contains resolvedSecrets map after Load
		version: version,
	}

	var err error
	app.iconData, err = resources.GetIcon()
	if err != nil {
		log.Printf("Warning: Failed to load embedded icon: %v", err)
	}

	ui.InitGlobalNotifications(cfg.UseNotifications, config.DefaultKeyringService, app.iconData) // Use AppName for consistency

	// Pass config reference and resolved secrets map to clipboard manager
	app.clipboardManager = clipboard.NewManager(cfg, cfg.GetResolvedSecrets(), app.onRevertStatusChange)

	// Pass config reference to hotkey manager
	app.hotkeyManager = hotkey.NewManager(cfg, app.onHotkeyTriggered, app.onRevertHotkey)

	// Add secret management callbacks to systray manager
	app.systrayManager = ui.NewSystrayManager(
		cfg,
		version,
		app.iconData,
		app.onReloadConfig, // Reloads config AND secrets
		app.onRestartApplication,
		app.onQuit,
		app.onRevertMenuItem,
		app.onOpenConfigFile,
		app.onViewLastDiffTriggered,
		app.onAddSecret,    // Added callback
		app.onListSecrets,  // Added callback
		app.onRemoveSecret, // Added callback
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
	// Start the systray manager (blocking call)
	a.systrayManager.Run()
}

// onHotkeyTriggered is called when a hotkey is pressed
func (a *Application) onHotkeyTriggered(hotkeyStr string, isReverse bool) {
	// clipboardManager uses its internal config reference and resolved secrets
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
			// Also clear the diff state in the UI when reverting
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

// onReloadConfig is called when the reload config menu item is clicked or triggered internally
func (a *Application) onReloadConfig() {
	log.Println("Reloading configuration and secrets...")

	// --- Preserve state across reload ---
	enabledStatus := make(map[string]bool)
	originalProfileCount := 0
	if a.config != nil && a.config.Profiles != nil {
		originalProfileCount = len(a.config.Profiles)
		for _, profile := range a.config.Profiles {
			enabledStatus[profile.Name] = profile.Enabled
		}
	} else {
		log.Println("Warning: Attempting to reload config, but current config is nil.")
	}

	configPath := ""
	if a.config != nil {
		configPath = a.config.GetConfigPath()
	}
	if configPath == "" {
		configPath = "config.json"
		log.Printf("Current config path is empty, attempting reload from default '%s'.", configPath)
		if _, errStat := os.Stat(configPath); os.IsNotExist(errStat) {
			log.Printf("Cannot reload config: Default config file '%s' does not exist.", configPath)
			ui.ShowNotification("Configuration Error", fmt.Sprintf("Config file '%s' not found.", configPath))
			return
		}
	}

	newConfig, err := config.Load(configPath)
	if err != nil {
		log.Printf("Error reloading configuration from '%s': %v", configPath, err)
		ui.ShowNotification("Configuration Error",
			fmt.Sprintf("Failed to reload configuration. Check %s and keychain access.", configPath))
		return
	}

	// --- Apply reloaded config and state ---
	a.config = newConfig

	for i, profile := range a.config.Profiles {
		if enabled, exists := enabledStatus[profile.Name]; exists {
			if i < len(a.config.Profiles) {
				a.config.Profiles[i].Enabled = enabled
			}
		}
	}

	profileStructureChanged := originalProfileCount != len(a.config.Profiles)
	if !profileStructureChanged && originalProfileCount > 0 {
		newProfileNames := make(map[string]bool)
		for _, profile := range a.config.Profiles {
			newProfileNames[profile.Name] = true
		}
		for name, enabled := range enabledStatus {
			if enabled && !newProfileNames[name] {
				profileStructureChanged = true
				break
			}
		}
		if !profileStructureChanged {
			for name := range newProfileNames {
				if _, exists := enabledStatus[name]; !exists {
					profileStructureChanged = true
					break
				}
			}
		}
	}

	log.Println("Configuration and secrets reloaded successfully.")

	a.hotkeyManager = hotkey.NewManager(a.config, a.onHotkeyTriggered, a.onRevertHotkey)
	if err := a.hotkeyManager.RegisterAll(); err != nil {
		log.Printf("Warning: Failed to register some hotkeys after reload: %v", err)
		ui.ShowNotification("Hotkey Registration Issue",
			fmt.Sprintf("Some hotkeys could not be registered after reload: %v", err))
	}

	if a.clipboardManager != nil {
		a.clipboardManager.UpdateResolvedSecrets(a.config.GetResolvedSecrets())
	} else {
		a.clipboardManager = clipboard.NewManager(a.config, a.config.GetResolvedSecrets(), a.onRevertStatusChange)
	}

	if a.systrayManager != nil {
		a.systrayManager.UpdateConfig(a.config)
		if profileStructureChanged {
			log.Println("Profile structure changed significantly. Restarting application is recommended for full menu update.")
			ui.ShowNotification("Configuration Reloaded",
				"Profile structure changed. Please use 'Restart Application' via menu to fully refresh UI.")
		} else {
			ui.ShowNotification("Configuration Reloaded",
				"Configuration and secrets updated successfully. Hotkeys have been refreshed.")
		}
	} else {
		ui.ShowNotification("Configuration Reloaded", "Configuration and secrets updated successfully.")
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
	configPath := ""
	if a.config != nil {
		configPath = a.config.GetConfigPath()
	}
	if configPath == "" {
		configPath = "config.json"
		log.Printf("Config path not found in current config, attempting to open default: %s", configPath)
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
		ui.ShowNotification("Error Opening File", fmt.Sprintf("Error checking config file '%s': %v", absPath, err))
		return
	} else {
		log.Printf("Config file exists at: %s", absPath)
	}

	log.Printf("Attempting to open config file using ui.OpenFileInDefaultApp with path: %s", absPath)
	err = ui.OpenFileInDefaultApp(absPath)
	if err != nil {
		log.Printf("Final error after trying open methods for config file '%s': %v", absPath, err)
		ui.ShowNotification("Error Opening File", fmt.Sprintf("Could not open config file '%s': %v", absPath, err))
	}
}

// --- Secret Management Handlers ---

// onAddSecret is called when the Add/Update Secret menu item is clicked
func (a *Application) onAddSecret() {
	log.Println("Add/Update Secret menu item clicked.")
	appName := config.DefaultKeyringService

	// === Step 1: Get Logical Name ===
	name, err := zenity.Entry("Step 1: Enter Logical Name\n(e.g., my_api_key, no spaces/special chars)",
		zenity.Title(appName+" - Add/Update Secret"),
	)
	if err != nil {
		if errors.Is(err, zenity.ErrCanceled) {
			log.Println("Add/Update Secret canceled by user (name entry).")
			ui.ShowNotification("Operation Canceled", "Add/Update Secret canceled.")
		} else {
			log.Printf("Error getting logical name via zenity: %v", err)
			ui.ShowNotification("Input Error", "Failed to get logical name input.")
		}
		return
	}
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, " {}[]()<>|=+*?^$\\./") {
		log.Printf("Invalid logical name entered: '%s'", name)
		ui.ShowNotification("Invalid Input", "Invalid logical name (empty or contains spaces/special chars). Aborted.")
		return
	}

	// === Step 2: Get Secret Value ===
	_, value, err := zenity.Password(
		zenity.Title(appName + " - Step 2: Enter Secret Value for '"+name+"'"),
		// No Username option needed here
	)
	if err != nil {
		if errors.Is(err, zenity.ErrCanceled) {
			log.Printf("Add/Update Secret canceled by user (value entry for '%s').", name)
			ui.ShowNotification("Operation Canceled", "Add/Update Secret canceled.")
		} else {
			log.Printf("Error getting secret value via zenity for '%s': %v", name, err)
			ui.ShowNotification("Input Error", "Failed to get secret value input.")
		}
		return
	}
	if value == "" {
		log.Printf("Add/Update Secret aborted: Empty secret value provided or dialog canceled for '%s'.", name)
		ui.ShowNotification("Add Secret Aborted", "Secret value cannot be empty or dialog canceled.")
		return
	}

	// === Store Secret (Error handling needed before optional steps) ===
	if a.config == nil {
		log.Println("Error: Cannot add secret, application config is nil.")
		ui.ShowNotification("Internal Error", "Application configuration not loaded.")
		return
	}
	err = a.config.AddSecretReference(name, value)
	if err != nil {
		log.Printf("Error adding/updating secret '%s': %v", name, err)
		ui.ShowNotification("Error", fmt.Sprintf("Failed to store secret '%s'. See logs.", name))
		return // Stop here if storing the secret failed
	} else {
		log.Printf("Secret '%s' updated in keychain and config.json.", name)
		// Don't notify yet, wait until optional steps are done or skipped
	}

	// === Step 3 (Optional): Ask to add replacement rule ===
	// Use zenity.Question which returns error (nil on OK, ErrCanceled on Cancel/Close)
	err = zenity.Question(
		fmt.Sprintf("Secret '%s' stored successfully.\n\nDo you want to create a basic replacement rule for it now?\n(Replaces occurrences of the secret with entered text)", name),
		zenity.Title(appName+" - Optional Step 3: Add Replacement Rule?"),
		zenity.InfoIcon, // Use Info or Question icon
		zenity.OKLabel("Yes, Add Rule"),
		zenity.CancelLabel("No, Just Store Secret"),
	)

	addRule := (err == nil) // User clicked "Yes" if err is nil

	if err != nil && !errors.Is(err, zenity.ErrCanceled) {
		// Log unexpected error, but treat as "No"
		log.Printf("Error showing 'Add Rule?' dialog: %v. Skipping rule creation.", err)
		addRule = false
	}

	if !addRule { // User chose No, or dialog was canceled/errored
		log.Println("Skipping optional replacement rule creation.")
		ui.ShowNotification("Secret Stored", fmt.Sprintf("Secret '%s' stored. Manual restart required to activate.", name))
		return // Finish workflow
	}

	// === Step 4 (Optional): Get Replacement String ===
	replaceWithString, err := zenity.Entry(
		fmt.Sprintf("Step 4: Enter text to replace '%s' with:\n(e.g., [REDACTED_KEY], MyPlaceholder)", name),
		zenity.Title(appName+" - Add Replacement Rule"),
	)
	if err != nil {
		// Handle cancel/error for replacement string entry
		if errors.Is(err, zenity.ErrCanceled) {
			log.Println("Rule creation canceled by user (replacement text entry).")
		} else {
			log.Printf("Error getting replacement text via zenity: %v", err)
		}
		ui.ShowNotification("Rule Creation Canceled", fmt.Sprintf("Secret '%s' stored, but rule creation canceled. Manual restart required for secret.", name))
		return
	}
	// Allow empty replacement string if desired

	// === Step 5 (Optional): Select Profile ===
	if a.config.Profiles == nil || len(a.config.Profiles) == 0 {
		log.Println("Cannot add replacement rule: No profiles found in configuration.")
		ui.ShowNotification("Cannot Add Rule", "No profiles exist. Please add a profile manually first.")
		ui.ShowNotification("Secret Stored", fmt.Sprintf("Secret '%s' stored. Manual restart required to activate.", name)) // Remind about secret
		return
	}

	profileNames := make([]string, len(a.config.Profiles))
	for i, p := range a.config.Profiles {
		profileNames[i] = p.Name
	}

	selectedProfileName, err := zenity.List(
		"Step 5: Select Profile to add the rule to:",
		profileNames,
		zenity.Title(appName+" - Add Replacement Rule"),
	)
	if err != nil {
		if errors.Is(err, zenity.ErrCanceled) {
			log.Println("Rule creation canceled by user (profile selection).")
		} else {
			log.Printf("Error getting profile selection via zenity list: %v", err)
		}
		ui.ShowNotification("Rule Creation Canceled", fmt.Sprintf("Secret '%s' stored, but rule creation canceled. Manual restart required for secret.", name))
		return
	}
	if selectedProfileName == "" { // Should not happen if list not empty, but check
		log.Println("Rule creation aborted: No profile selected.")
		ui.ShowNotification("Rule Creation Canceled", fmt.Sprintf("Secret '%s' stored, but rule creation canceled (no profile selected). Manual restart required for secret.", name))
		return
	}

	// === Add the Rule to Config and Save ===
	newReplacement := config.Replacement{
		Regex:        fmt.Sprintf("{{%s}}", name), // Use placeholder for regex
		ReplaceWith:  replaceWithString,
		PreserveCase: false, // Sensible default for secrets
		ReverseWith:  "",    // Default to empty
	}

	// Find the target profile index
	targetProfileIndex := -1
	for i, p := range a.config.Profiles {
		if p.Name == selectedProfileName {
			targetProfileIndex = i
			break
		}
	}

	if targetProfileIndex == -1 { // Should be impossible if List worked correctly
		log.Printf("Internal Error: Selected profile '%s' not found in config.", selectedProfileName)
		ui.ShowNotification("Internal Error", "Selected profile not found.")
		ui.ShowNotification("Secret Stored", fmt.Sprintf("Secret '%s' stored. Manual restart required to activate.", name)) // Remind about secret
		return
	}

	// Append the replacement to the profile
	a.config.Profiles[targetProfileIndex].Replacements = append(a.config.Profiles[targetProfileIndex].Replacements, newReplacement)
	log.Printf("Prepared replacement rule for secret '%s' to be added to profile '%s'.", name, selectedProfileName)

	// Save the config again with the added rule
	err = a.config.Save()
	if err != nil {
		log.Printf("Error saving config after adding replacement rule for secret '%s': %v", name, err)
		ui.ShowNotification("Save Error", "Failed to save config after adding replacement rule.")
		// Don't roll back the rule addition in memory, just report save failure
	} else {
		log.Printf("Successfully added replacement rule for '%s' to profile '%s' and saved config.", name, selectedProfileName)
		// Final success notification
		ui.ShowNotification("Secret & Rule Added", fmt.Sprintf("Secret '%s' stored and rule added to profile '%s'. Manual restart required to activate.", name, selectedProfileName))
	}
}

// onListSecrets is called when the List Secrets menu item is clicked
func (a *Application) onListSecrets() {
	log.Println("List Secrets menu item clicked.")
	if a.config == nil {
		log.Println("Error: Cannot list secrets, application config is nil.")
		ui.ShowNotification("Internal Error", "Application configuration not loaded.")
		return
	}
	names := a.config.GetSecretNames()
	var message string
	if len(names) == 0 {
		message = "No secrets are currently managed in config.json."
		log.Println(message)
		zenity.Info(message, zenity.Title(config.DefaultKeyringService+" - Managed Secrets"), zenity.InfoIcon)
	} else {
		messageBody := "- " + strings.Join(names, "\n- ")
		messageLog := fmt.Sprintf("Managed secrets (%d total):\n%s", len(names), messageBody)
		log.Printf("Listing managed secrets:\n%s", messageLog)
		message = fmt.Sprintf("Found %d managed secret(s). See log for names.", len(names))
		ui.ShowNotification("Managed Secrets", message)
		zenity.Info(messageLog, zenity.Title(config.DefaultKeyringService+" - Managed Secrets"), zenity.InfoIcon)
	}
}

// onRemoveSecret is called when the Remove Secret menu item is clicked
func (a *Application) onRemoveSecret() {
	log.Println("Remove Secret menu item clicked.")
	appName := config.DefaultKeyringService

	if a.config == nil {
		log.Println("Error: Cannot remove secret, application config is nil.")
		ui.ShowNotification("Internal Error", "Application configuration not loaded.")
		return
	}

	names := a.config.GetSecretNames()
	if len(names) == 0 {
		log.Println("No secrets to remove.")
		ui.ShowNotification("Remove Secret", "No secrets are currently managed.")
		zenity.Info("No secrets are currently managed.", zenity.Title(appName+" - Remove Secret"), zenity.InfoIcon)
		return
	}

	nameToRemove, err := zenity.List(
		"Select secret to remove:",
		names,
		zenity.Title(appName+" - Remove Secret"),
	)
	if err != nil {
		if errors.Is(err, zenity.ErrCanceled) {
			log.Println("Remove secret canceled by user.")
			ui.ShowNotification("Operation Canceled", "Remove secret canceled.")
		} else {
			log.Printf("Error getting selection via zenity list: %v", err)
			ui.ShowNotification("Input Error", "Failed to get secret selection.")
		}
		return
	}
	if nameToRemove == "" {
		log.Println("Remove secret aborted: No secret selected.")
		ui.ShowNotification("Operation Canceled", "No secret selected for removal.")
		return
	}

	err = zenity.Question(
		fmt.Sprintf("Are you sure you want to remove the secret '%s'?\n\nThis will remove it from the OS keychain and the application's config. This cannot be undone.", nameToRemove),
		zenity.Title(appName+" - Confirm Removal"),
		zenity.WarningIcon,
		zenity.OKLabel("Remove"),
		zenity.CancelLabel("Cancel"),
	)

	confirmed := (err == nil) // Confirmed if error is nil

	if err != nil && !errors.Is(err, zenity.ErrCanceled) { // Log unexpected errors
		log.Printf("Error displaying confirmation dialog: %v", err)
		ui.ShowNotification("Dialog Error", "Failed to show confirmation dialog.")
		return // Abort on unexpected error
	}

	if !confirmed { // Handle Cancel or other errors treated as cancel
		log.Printf("Removal of secret '%s' canceled by user or dialog error.", nameToRemove)
		ui.ShowNotification("Operation Canceled", "Secret removal canceled.")
		return
	}

	// Remove Secret
	err = a.config.RemoveSecretReference(nameToRemove)
	if err != nil {
		log.Printf("Error removing secret '%s': %v", nameToRemove, err)
		ui.ShowNotification("Error", fmt.Sprintf("Failed to remove secret '%s'. See logs.", nameToRemove))
	} else {
		log.Printf("Secret '%s' removed from config and potentially keyring.", nameToRemove)
		ui.ShowNotification("Secret Removed", fmt.Sprintf("Secret '%s' removed. Manual restart required for change to take effect.", nameToRemove))
	}
}

// --- End Secret Management Handlers ---