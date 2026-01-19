// ==== internal/app/app.go ====
package app

import (
	"errors" // Needed for zenity error checking
	"fmt"    // Needed for Sprintf
	"log"
	"os" // Needed by os.Exit in RestartApplication call chain
	"path/filepath"
	"regexp" // <-- Import regexp
	"strings"
	//"time" // <-- Import time (Needed again for profile name generation)

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

	// ui.InitGlobalNotifications is now called in main.go AFTER config load succeeds.

	// Pass config reference and resolved secrets map to clipboard manager
	app.clipboardManager = clipboard.NewManager(cfg, cfg.GetResolvedSecrets(), app.onRevertStatusChange)

	// Pass config reference to hotkey manager
	app.hotkeyManager = hotkey.NewManager(cfg, app.onHotkeyTriggered, app.onRevertHotkey)

	// Add secret management and simple rule callbacks to systray manager
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
		app.onAddSecret,
		app.onListSecrets,
		app.onRemoveSecret,
		app.onAddSimpleRule, // <-- Pass the new callback
	)

	return app
}

// Run starts the application
func (a *Application) Run() {
	if err := a.hotkeyManager.RegisterAll(); err != nil {
		errMsg := fmt.Sprintf("Some hotkeys could not be registered: %v", err)
		log.Printf("Warning: Failed to register some hotkeys: %v", err)
		ui.ShowAdminNotification(ui.LevelWarn, "Hotkey Registration Issue", errMsg) // <<< CHANGED
	}
	// Start the systray manager (blocking call)
	a.systrayManager.Run()
}

// onHotkeyTriggered is called when a hotkey is pressed
func (a *Application) onHotkeyTriggered(hotkeyStr string, isReverse bool) {
	// clipboardManager uses its internal config reference and resolved secrets
	message, changedForDiff := a.clipboardManager.ProcessClipboard(hotkeyStr, isReverse)
	if message != "" {
		// This is the specific replacement notification
		ui.ShowReplacementNotification("Clipboard Updated", message) // <<< CHANGED
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
		ui.ShowAdminNotification(ui.LevelInfo, "View Changes", "No changes recorded from the last operation.") // <<< CHANGED
		if a.systrayManager != nil {
			a.systrayManager.UpdateViewLastDiffStatus(false)
		}
		return
	}
	log.Println("View Last Change Details clicked, showing diff viewer.")
	contextLines := a.config.GetDiffContextLines()
	ui.ShowDiffViewer(original, modified, contextLines)
}

// onRevertHotkey is called when the revert hotkey is pressed
func (a *Application) onRevertHotkey() {
	if a.clipboardManager.RestoreOriginalClipboard() {
		// Consider if this is Admin or Replacement context. Let's say Admin Info.
		ui.ShowAdminNotification(ui.LevelInfo, "Clipboard Reverted", "Original clipboard content has been restored.") // <<< CHANGED
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
			errMsg := fmt.Sprintf("Config file '%s' not found.", configPath)
			log.Printf("Cannot reload config: %s", errMsg)
			ui.ShowAdminNotification(ui.LevelError, "Configuration Error", errMsg) // <<< CHANGED (Error level)
			return
		}
	}

	newConfig, err := config.Load(configPath)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to reload configuration. Check %s and keychain access. Error: %v", configPath, err)
		log.Printf("Error reloading configuration from '%s': %v", configPath, err)
		ui.ShowAdminNotification(ui.LevelError, "Configuration Error", errMsg) // <<< CHANGED (Error level)
		return
	}

	// --- Apply reloaded config and state ---
	a.config = newConfig

	// Restore enabled status for profiles that still exist by name
	if a.config.Profiles != nil {
		for i := range a.config.Profiles { // Iterate over the NEW config profiles
			profileName := a.config.Profiles[i].Name
			if enabled, exists := enabledStatus[profileName]; exists {
				a.config.Profiles[i].Enabled = enabled
				log.Printf("Restored enabled status (%t) for profile '%s'", enabled, profileName)
			} else {
				log.Printf("Profile '%s' is new or renamed, keeping its default enabled status (%t)", profileName, a.config.Profiles[i].Enabled)
			}
		}
	}

	// Detect significant profile structure changes (additions/removals)
	profileStructureChanged := originalProfileCount != len(a.config.Profiles)
	if !profileStructureChanged && originalProfileCount > 0 && a.config.Profiles != nil {
		newProfileNames := make(map[string]bool)
		for _, profile := range a.config.Profiles {
			newProfileNames[profile.Name] = true
		}
		// Check if any old profiles are missing in the new set
		for name := range enabledStatus {
			if !newProfileNames[name] {
				profileStructureChanged = true
				break
			}
		}
		// Check if any new profiles were added
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

	// Re-register hotkeys based on the new config
	a.hotkeyManager = hotkey.NewManager(a.config, a.onHotkeyTriggered, a.onRevertHotkey)
	if err := a.hotkeyManager.RegisterAll(); err != nil {
		errMsg := fmt.Sprintf("Some hotkeys could not be registered after reload: %v", err)
		log.Printf("Warning: Failed to register some hotkeys after reload: %v", err)
		ui.ShowAdminNotification(ui.LevelWarn, "Hotkey Registration Issue", errMsg) // <<< CHANGED (Warn level)
	} else {
		log.Println("Hotkeys re-registered successfully after config reload.")
	}

	// Update clipboard manager with new secrets and config reference
	if a.clipboardManager != nil {
		a.clipboardManager.UpdateResolvedSecrets(a.config.GetResolvedSecrets())
		a.clipboardManager.UpdateConfig(a.config) // Assuming clipboard manager has UpdateConfig
	} else {
		// Should not happen normally, but handle defensively
		a.clipboardManager = clipboard.NewManager(a.config, a.config.GetResolvedSecrets(), a.onRevertStatusChange)
	}

	// Update systray manager with the new config reference
	if a.systrayManager != nil {
		a.systrayManager.UpdateConfig(a.config) // Update systray internal config ref
		if profileStructureChanged {
			msg := "Profile structure changed. Please use 'Restart Application' via menu to fully refresh UI."
			log.Println("Profile structure changed significantly. Restarting application is recommended for full menu update.")
			ui.ShowAdminNotification(ui.LevelWarn, "Configuration Reloaded", msg) // <<< CHANGED (Warn level)
		} else {
			msg := "Configuration and secrets updated successfully. Hotkeys have been refreshed."
			ui.ShowAdminNotification(ui.LevelInfo, "Configuration Reloaded", msg) // <<< CHANGED (Info level)
		}
	} else {
		// Should not happen if app initialization is correct
		ui.ShowAdminNotification(ui.LevelInfo, "Configuration Reloaded", "Configuration and secrets updated successfully.") // <<< CHANGED (Info level)
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
		errMsg := fmt.Sprintf("Config file not found: %s", absPath)
		log.Printf("Error: %s", errMsg)
		ui.ShowAdminNotification(ui.LevelWarn, "Error Opening File", errMsg) // <<< CHANGED (Warn level)
		return
	} else if err != nil {
		errMsg := fmt.Sprintf("Error checking config file '%s': %v", absPath, err)
		log.Printf("Error checking config file status at path '%s': %v", absPath, err)
		ui.ShowAdminNotification(ui.LevelWarn, "Error Opening File", errMsg) // <<< CHANGED (Warn level)
		return
	} else {
		log.Printf("Config file exists at: %s", absPath)
	}

	log.Printf("Attempting to open config file using ui.OpenFileInDefaultApp with path: %s", absPath)
	err = ui.OpenFileInDefaultApp(absPath)
	if err != nil {
		errMsg := fmt.Sprintf("Could not open config file '%s': %v", absPath, err)
		log.Printf("Final error after trying open methods for config file '%s': %v", absPath, err)
		ui.ShowAdminNotification(ui.LevelWarn, "Error Opening File", errMsg) // <<< CHANGED (Warn level)
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
			ui.ShowAdminNotification(ui.LevelInfo, "Operation Canceled", "Add/Update Secret canceled.") // <<< CHANGED (Info level)
		} else {
			log.Printf("Error getting logical name via zenity: %v", err)
			ui.ShowAdminNotification(ui.LevelWarn, "Input Error", "Failed to get logical name input.") // <<< CHANGED (Warn level)
		}
		return
	}
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, " {}[]()<>|=+*?^$\\./") {
		errMsg := fmt.Sprintf("Invalid logical name (empty or contains spaces/special chars): '%s'. Aborted.", name)
		log.Printf("Invalid logical name entered: '%s'", name)
		ui.ShowAdminNotification(ui.LevelWarn, "Invalid Input", errMsg) // <<< CHANGED (Warn level)
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
			ui.ShowAdminNotification(ui.LevelInfo, "Operation Canceled", "Add/Update Secret canceled.") // <<< CHANGED (Info level)
		} else {
			log.Printf("Error getting secret value via zenity for '%s': %v", name, err)
			ui.ShowAdminNotification(ui.LevelWarn, "Input Error", "Failed to get secret value input.") // <<< CHANGED (Warn level)
		}
		return
	}
	if value == "" {
		log.Printf("Add/Update Secret aborted: Empty secret value provided or dialog canceled for '%s'.", name)
		ui.ShowAdminNotification(ui.LevelWarn, "Add Secret Aborted", "Secret value cannot be empty or dialog canceled.") // <<< CHANGED (Warn level)
		return
	}

	// === Store Secret (Error handling needed before optional steps) ===
	if a.config == nil {
		log.Println("Error: Cannot add secret, application config is nil.")
		ui.ShowAdminNotification(ui.LevelError, "Internal Error", "Application configuration not loaded.") // <<< CHANGED (Error level)
		return
	}
	err = a.config.AddSecretReference(name, value)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to store secret '%s'. See logs. Error: %v", name, err)
		log.Printf("Error adding/updating secret '%s': %v", name, err)
		ui.ShowAdminNotification(ui.LevelError, "Error", errMsg) // <<< CHANGED (Error level)
		return // Stop here if storing the secret failed
	} else {
		log.Printf("Secret '%s' updated in keychain and config.json.", name)
		// Don't notify yet, wait until optional steps are done or skipped
	}

	// === Step 3 (Optional): Ask to add replacement rule ===
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
		ui.ShowAdminNotification(ui.LevelInfo, "Secret Stored", fmt.Sprintf("Secret '%s' stored. Manual restart required to activate.", name)) // <<< CHANGED (Info level)
		return // Finish workflow
	}

	// === Step 4 (Optional): Get Replacement String ===
	replaceWithString, err := zenity.Entry(
		fmt.Sprintf("Step 4: Enter text to replace '{{%s}}' with:\n(e.g., [REDACTED_KEY], MyPlaceholder)", name), // Corrected placeholder format in prompt
		zenity.Title(appName+" - Add Replacement Rule"),
	)
	if err != nil {
		// Handle cancel/error for replacement string entry
		if errors.Is(err, zenity.ErrCanceled) {
			log.Println("Rule creation canceled by user (replacement text entry).")
		} else {
			log.Printf("Error getting replacement text via zenity: %v", err)
		}
		ui.ShowAdminNotification(ui.LevelInfo, "Rule Creation Canceled", fmt.Sprintf("Secret '%s' stored, but rule creation canceled. Manual restart required for secret.", name)) // <<< CHANGED (Info level)
		return
	}
	// Allow empty replacement string if desired

	// === Step 5 (Optional): Select Profile ===
	if a.config.Profiles == nil || len(a.config.Profiles) == 0 {
		log.Println("Cannot add replacement rule: No profiles found in configuration.")
		ui.ShowAdminNotification(ui.LevelWarn, "Cannot Add Rule", "No profiles exist. Please add a profile manually first.") // <<< CHANGED (Warn level)
		ui.ShowAdminNotification(ui.LevelInfo, "Secret Stored", fmt.Sprintf("Secret '%s' stored. Manual restart required to activate.", name)) // Remind about secret // <<< CHANGED (Info level)
		return
	}

	profileNames := make([]string, len(a.config.Profiles))
	profileMap := make(map[string]int) // Map name back to index
	for i, p := range a.config.Profiles {
		profileNames[i] = p.Name
		profileMap[p.Name] = i
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
		ui.ShowAdminNotification(ui.LevelInfo, "Rule Creation Canceled", fmt.Sprintf("Secret '%s' stored, but rule creation canceled. Manual restart required for secret.", name)) // <<< CHANGED (Info level)
		return
	}
	if selectedProfileName == "" { // Should not happen if list not empty, but check
		log.Println("Rule creation aborted: No profile selected.")
		ui.ShowAdminNotification(ui.LevelInfo, "Rule Creation Canceled", fmt.Sprintf("Secret '%s' stored, but rule creation canceled (no profile selected). Manual restart required for secret.", name)) // <<< CHANGED (Info level)
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
	targetProfileIndex, found := profileMap[selectedProfileName]
	if !found { // Should be impossible if List worked correctly
		log.Printf("Internal Error: Selected profile '%s' not found in config map.", selectedProfileName)
		ui.ShowAdminNotification(ui.LevelError, "Internal Error", "Selected profile not found.") // <<< CHANGED (Error level)
		ui.ShowAdminNotification(ui.LevelInfo, "Secret Stored", fmt.Sprintf("Secret '%s' stored. Manual restart required to activate.", name)) // Remind about secret // <<< CHANGED (Info level)
		return
	}

	// Append the replacement to the profile
	a.config.Profiles[targetProfileIndex].Replacements = append(a.config.Profiles[targetProfileIndex].Replacements, newReplacement)
	log.Printf("Prepared replacement rule for secret '%s' to be added to profile '%s'.", name, selectedProfileName)

	// Save the config again with the added rule
	err = a.config.Save()
	if err != nil {
		errMsg := fmt.Sprintf("Failed to save config after adding replacement rule. Error: %v", err)
		log.Printf("Error saving config after adding replacement rule for secret '%s': %v", name, err)
		ui.ShowAdminNotification(ui.LevelError, "Save Error", errMsg) // <<< CHANGED (Error level)
		// Attempt to roll back in-memory change
		prof := &a.config.Profiles[targetProfileIndex]
		if len(prof.Replacements) > 0 {
			prof.Replacements = prof.Replacements[:len(prof.Replacements)-1]
		}
	} else {
		log.Printf("Successfully added replacement rule for '%s' to profile '%s' and saved config.", name, selectedProfileName)
		// Final success notification
		ui.ShowAdminNotification(ui.LevelInfo, "Secret & Rule Added", fmt.Sprintf("Secret '%s' stored and rule added to profile '%s'. Manual restart required to activate.", name, selectedProfileName)) // <<< CHANGED (Info level)
	}
}

// onListSecrets is called when the List Secrets menu item is clicked
func (a *Application) onListSecrets() {
	log.Println("List Secrets menu item clicked.")
	if a.config == nil {
		log.Println("Error: Cannot list secrets, application config is nil.")
		ui.ShowAdminNotification(ui.LevelError, "Internal Error", "Application configuration not loaded.") // <<< CHANGED (Error level)
		return
	}
	names := a.config.GetSecretNames()
	var message string
	var dialogMessage string
	if len(names) == 0 {
		message = "No secrets are currently managed in config.json."
		dialogMessage = message
		log.Println(message)
	} else {
		messageBody := "- " + strings.Join(names, "\n- ")
		logMessage := fmt.Sprintf("Managed secrets (%d total):\n%s", len(names), messageBody) // Detailed for log
		dialogMessage = logMessage                                                             // Show details in dialog too
		message = fmt.Sprintf("Found %d managed secret(s).", len(names))                       // Brief for notification
		log.Printf("Listing managed secrets:\n%s", logMessage)                                 // Log details
	}
	// Show brief notification first, then detailed dialog
	ui.ShowAdminNotification(ui.LevelInfo, "Managed Secrets", message) // <<< CHANGED (Info level)
	zenity.Info(dialogMessage, zenity.Title(config.DefaultKeyringService+" - Managed Secrets"), zenity.InfoIcon)
}

// onRemoveSecret is called when the Remove Secret menu item is clicked
func (a *Application) onRemoveSecret() {
	log.Println("Remove Secret menu item clicked.")
	appName := config.DefaultKeyringService

	if a.config == nil {
		log.Println("Error: Cannot remove secret, application config is nil.")
		ui.ShowAdminNotification(ui.LevelError, "Internal Error", "Application configuration not loaded.") // <<< CHANGED (Error level)
		return
	}

	names := a.config.GetSecretNames()
	if len(names) == 0 {
		log.Println("No secrets to remove.")
		msg := "No secrets are currently managed."
		ui.ShowAdminNotification(ui.LevelInfo, "Remove Secret", msg) // <<< CHANGED (Info level)
		zenity.Info(msg, zenity.Title(appName+" - Remove Secret"), zenity.InfoIcon)
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
			ui.ShowAdminNotification(ui.LevelInfo, "Operation Canceled", "Remove secret canceled.") // <<< CHANGED (Info level)
		} else {
			log.Printf("Error getting selection via zenity list: %v", err)
			ui.ShowAdminNotification(ui.LevelWarn, "Input Error", "Failed to get secret selection.") // <<< CHANGED (Warn level)
		}
		return
	}
	if nameToRemove == "" {
		log.Println("Remove secret aborted: No secret selected.")
		ui.ShowAdminNotification(ui.LevelInfo, "Operation Canceled", "No secret selected for removal.") // <<< CHANGED (Info level)
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
		ui.ShowAdminNotification(ui.LevelWarn, "Dialog Error", "Failed to show confirmation dialog.") // <<< CHANGED (Warn level)
		return // Abort on unexpected error
	}

	if !confirmed { // Handle Cancel or other errors treated as cancel
		log.Printf("Removal of secret '%s' canceled by user or dialog error.", nameToRemove)
		ui.ShowAdminNotification(ui.LevelInfo, "Operation Canceled", "Secret removal canceled.") // <<< CHANGED (Info level)
		return
	}

	// Remove Secret
	err = a.config.RemoveSecretReference(nameToRemove)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to remove secret '%s'. See logs. Error: %v", nameToRemove, err)
		log.Printf("Error removing secret '%s': %v", nameToRemove, err)
		ui.ShowAdminNotification(ui.LevelError, "Error", errMsg) // <<< CHANGED (Error level)
	} else {
		log.Printf("Secret '%s' removed from config and potentially keyring.", nameToRemove)
		ui.ShowAdminNotification(ui.LevelInfo, "Secret Removed", fmt.Sprintf("Secret '%s' removed. Manual restart required for change to take effect.", nameToRemove)) // <<< CHANGED (Info level)
	}
}

// --- Add Simple Rule Handler (Corrected using separate dialogs) ---

// onAddSimpleRule handles the user flow for adding a 1:1 replacement rule
func (a *Application) onAddSimpleRule() {
	log.Println("Add Simple Rule menu item clicked.")
	appName := config.DefaultKeyringService // Use for dialog titles

	// === Step 1: Check for existing profiles ===
	if a.config == nil || a.config.Profiles == nil || len(a.config.Profiles) == 0 {
		log.Println("Error: Cannot add rule, no profiles found in configuration.")
		zenity.Error("No profiles found. Please add a profile manually in config.json first.",
			zenity.Title(appName+" - Error"),
			zenity.ErrorIcon)
		// No notification needed here as zenity shows the error directly
		return
	}

	// === Step 2: Select Profile ===
	profileNames := make([]string, len(a.config.Profiles))
	profileMap := make(map[string]int) // Map name back to index
	for i, p := range a.config.Profiles {
		profileNames[i] = p.Name
		profileMap[p.Name] = i
	}

	selectedProfileName, err := zenity.List(
		"Step 1: Select Profile to add the rule to:", // Renamed step in title
		profileNames,
		zenity.Title(appName+" - Add Simple Rule"),
		zenity.Height(300), // Adjust height if needed
	)
	if err != nil {
		if errors.Is(err, zenity.ErrCanceled) {
			log.Println("Add simple rule canceled by user (profile selection).")
			ui.ShowAdminNotification(ui.LevelInfo, "Operation Canceled", "Add simple rule canceled.") // <<< CHANGED (Info level)
		} else {
			log.Printf("Error getting profile selection via zenity list: %v", err)
			ui.ShowAdminNotification(ui.LevelWarn, "Input Error", "Failed to get profile selection.") // <<< CHANGED (Warn level)
		}
		return
	}
	if selectedProfileName == "" { // Should not happen if list not empty, but check
		log.Println("Add simple rule aborted: No profile selected.")
		ui.ShowAdminNotification(ui.LevelInfo, "Operation Canceled", "No profile selected.") // <<< CHANGED (Info level)
		return
	}

	// === Step 3: Get Source Text ===
	sourceText, err := zenity.Entry(
		"Step 2: Enter Source Text\n(Text to find and replace. Special characters will be treated literally.)",
		zenity.Title(appName+" - Add Simple Rule"),
		zenity.DisallowEmpty(), // Ensure source text is not empty
	)
	if err != nil {
		if errors.Is(err, zenity.ErrCanceled) {
			log.Println("Add simple rule canceled by user (source text entry).")
			ui.ShowAdminNotification(ui.LevelInfo, "Operation Canceled", "Add simple rule canceled.") // <<< CHANGED (Info level)
		} else {
			log.Printf("Error getting source text via zenity: %v", err)
			ui.ShowAdminNotification(ui.LevelWarn, "Input Error", "Failed to get source text input.") // <<< CHANGED (Warn level)
		}
		return
	}
	// No need for explicit empty check as zenity.DisallowEmpty handles it

	// === Step 4: Get Replacement Text ===
	replacementText, err := zenity.Entry(
		"Step 3: Enter Replacement Text\n(Text to replace the source text with. Can be empty.)",
		zenity.Title(appName+" - Add Simple Rule"),
		// Allow empty replacement text
	)
	if err != nil {
		if errors.Is(err, zenity.ErrCanceled) {
			log.Println("Add simple rule canceled by user (replacement text entry).")
			ui.ShowAdminNotification(ui.LevelInfo, "Operation Canceled", "Add simple rule canceled.") // <<< CHANGED (Info level)
		} else {
			log.Printf("Error getting replacement text via zenity: %v", err)
			ui.ShowAdminNotification(ui.LevelWarn, "Input Error", "Failed to get replacement text input.") // <<< CHANGED (Warn level)
		}
		return
	}

	// === Step 5: Ask about Case Sensitivity ===
	err = zenity.Question(
		"Step 4: Make this rule case-insensitive?\n(Treat 'A' and 'a' as the same)",
		zenity.Title(appName+" - Add Simple Rule"),
		zenity.QuestionIcon,
		zenity.OKLabel("Yes"),
		zenity.CancelLabel("No"), // "No" corresponds to ErrCanceled
	)

	caseInsensitive := false
	if err == nil {
		// User clicked "Yes"
		caseInsensitive = true
		log.Println("Case-insensitive option selected.")
	} else if errors.Is(err, zenity.ErrCanceled) {
		// User clicked "No"
		caseInsensitive = false
		log.Println("Case-sensitive option selected.")
	} else {
		// Unexpected error during question dialog
		log.Printf("Error getting case sensitivity preference via zenity: %v", err)
		ui.ShowAdminNotification(ui.LevelWarn, "Input Error", "Failed to get case sensitivity preference.") // <<< CHANGED (Warn level)
		return
	}

	// === Step 6: Construct Rule ===
	escapedSourceText := regexp.QuoteMeta(sourceText)
	regexString := escapedSourceText
	if caseInsensitive {
		regexString = "(?i)" + escapedSourceText
	}

	newRule := config.Replacement{
		Regex:        regexString,
		ReplaceWith:  replacementText,
		PreserveCase: false, // Keep false for simple 1:1 rules
		ReverseWith:  "",    // Not applicable
	}

	log.Printf("Constructed new rule: Regex='%s', ReplaceWith='%s'", newRule.Regex, newRule.ReplaceWith)

	// === Step 7: Add Rule to Config and Save ===
	targetProfileIndex, found := profileMap[selectedProfileName]
	if !found { // Should be impossible if List worked correctly
		log.Printf("Internal Error: Selected profile '%s' not found in internal map.", selectedProfileName)
		ui.ShowAdminNotification(ui.LevelError, "Internal Error", "Selected profile could not be found.") // <<< CHANGED (Error level)
		return
	}

	// Append the replacement to the profile
	a.config.Profiles[targetProfileIndex].Replacements = append(a.config.Profiles[targetProfileIndex].Replacements, newRule)
	log.Printf("Prepared simple replacement rule to be added to profile '%s'.", selectedProfileName)

	// Save the config
	err = a.config.Save()
	if err != nil {
		errMsg := fmt.Sprintf("Failed to save config after adding rule. Error: %v", err)
		log.Printf("Error saving config after adding simple rule to profile '%s': %v", selectedProfileName, err)
		ui.ShowAdminNotification(ui.LevelError, "Save Error", errMsg) // <<< CHANGED (Error level)
		// Attempt to roll back the change in memory
		prof := &a.config.Profiles[targetProfileIndex]
		if len(prof.Replacements) > 0 {
			prof.Replacements = prof.Replacements[:len(prof.Replacements)-1]
		}
	} else {
		log.Printf("Successfully added simple rule to profile '%s' and saved config.", selectedProfileName)
		// Final success notification
		ui.ShowAdminNotification(ui.LevelInfo, "Rule Added", fmt.Sprintf("Rule added to profile '%s'. Use 'Reload Configuration' to apply.", selectedProfileName)) // <<< CHANGED (Info level)
	}
}

// --- End Add Simple Rule Handler ---