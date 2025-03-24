package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	// embed package (Go 1.16+)
	_ "embed"

	"github.com/atotto/clipboard"
	"github.com/gen2brain/beeep"
	"github.com/getlantern/systray"
	"github.com/go-toast/toast"
	"golang.design/x/hotkey"
)

const version = "v1.4.0" // Application version updated for multiple profiles

// ---------------------------------------------------------------------------
// 1. Embed the icon.ico for the tray and EXE icon.
// ---------------------------------------------------------------------------

//go:embed icon.ico
var embeddedIcon []byte

// writeTempIcon writes the embedded icon (icon.ico) to a temporary file
// and returns its absolute path. This is used as a fallback for toast notifications.
func writeTempIcon() (string, error) {
	tmpFile, err := ioutil.TempFile("", "icon-*.ico")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()
	if _, err := tmpFile.Write(embeddedIcon); err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(tmpFile.Name())
	if err != nil {
		return tmpFile.Name(), nil
	}
	return absPath, nil
}

// ---------------------------------------------------------------------------
// 2. Configuration & Replacement Rules
//    The configuration is loaded from an external config.json file.
// ---------------------------------------------------------------------------

// ProfileConfig represents a single regex replacement profile
type ProfileConfig struct {
	Name         string        `json:"name"`         // Display name for the profile
	Enabled      bool          `json:"enabled"`      // Whether the profile is active
	Hotkey       string        `json:"hotkey"`       // Hotkey combination to trigger this profile
	Replacements []Replacement `json:"replacements"` // Regex replacement rules for this profile
}

// Config holds the application configuration
type Config struct {
	UseNotifications   bool            `json:"use_notifications"`   // Whether to show notifications
	TemporaryClipboard bool            `json:"temporary_clipboard"` // Whether to store original clipboard
	AutomaticReversion bool            `json:"automatic_reversion"` // Whether to revert clipboard after paste
	Profiles           []ProfileConfig `json:"profiles"`            // List of replacement profiles

	// Legacy support fields (for backward compatibility)
	Hotkey       string        `json:"hotkey,omitempty"`       // Legacy hotkey field
	Replacements []Replacement `json:"replacements,omitempty"` // Legacy replacements field
}

// Replacement represents one regex replacement rule
type Replacement struct {
	Regex       string `json:"regex"`
	ReplaceWith string `json:"replace_with"`
}

var config Config

// loadConfig reads and parses the configuration file with backward compatibility
func loadConfig() error {
	data, err := ioutil.ReadFile("config.json")
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		return err
	}

	// Handle backward compatibility - migrate from legacy format to profiles
	if config.Hotkey != "" && len(config.Replacements) > 0 && len(config.Profiles) == 0 {
		// Convert old format to new format with a "Default" profile
		config.Profiles = []ProfileConfig{
			{
				Name:         "Default",
				Enabled:      true,
				Hotkey:       config.Hotkey,
				Replacements: config.Replacements,
			},
		}

		// Clear legacy fields to avoid confusion
		config.Hotkey = ""
		config.Replacements = nil

		// Save the migrated config
		saveConfig()
	}

	return nil
}

// saveConfig writes the current configuration back to the config.json file
func saveConfig() error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile("config.json", data, 0644)
}

// ---------------------------------------------------------------------------
// 3. Notification Function (using go-toast on Windows)
// ---------------------------------------------------------------------------

// showNotification displays a desktop notification.
// On Windows, it uses go-toast.
// It first checks for an external icon.png (high quality) and, if found,
// uses its absolute path. If not found, it falls back to the embedded icon.
// On non-Windows platforms, it falls back to beeep.
func showNotification(title, message string) {
	if !config.UseNotifications {
		return
	}
	if runtime.GOOS == "windows" {
		var iconPathForToast string

		// Try to use external icon.png for better quality.
		if _, err := os.Stat("icon.png"); err == nil {
			wd, err := os.Getwd()
			if err != nil {
				iconPathForToast = "icon.png"
			} else {
				iconPathForToast = filepath.Join(wd, "icon.png")
			}
			log.Println("Using external icon.png for toast notifications from:", iconPathForToast)
		} else {
			log.Println("icon.png not found; using fallback embedded icon.")
			var err2 error
			iconPathForToast, err2 = writeTempIcon()
			if err2 != nil {
				log.Printf("Error writing temporary icon: %v", err2)
				iconPathForToast = "" // fallback: no icon
			} else {
				// Remove the temporary file after 10 seconds.
				time.AfterFunc(10*time.Second, func() { os.Remove(iconPathForToast) })
			}
		}

		notification := toast.Notification{
			AppID:   "Clipboard Regex Replace", // Ensure this matches a registered AppUserModelID if needed.
			Title:   title,
			Message: message,
			Icon:    iconPathForToast,
		}
		err := notification.Push()
		if err != nil {
			log.Printf("Error showing toast notification: %v", err)
		} else {
			log.Println("Toast notification sent successfully.")
		}
	} else {
		if err := beeep.Notify(title, message, ""); err != nil {
			log.Printf("Error showing beeep notification: %v", err)
		} else {
			log.Println("Beeep notification sent successfully.")
		}
	}
}

// ---------------------------------------------------------------------------
// 4. Clipboard Processing, Temporary Storage & Paste Simulation
// ---------------------------------------------------------------------------

// Global variables for clipboard handling and hotkeys
var previousClipboard string
var lastTransformedClipboard string
var miRevert *systray.MenuItem
var registeredHotkeys map[string]*hotkey.Hotkey

// replaceClipboardText reads the clipboard text, applies regex replacements from all
// enabled profiles that match the given hotkey, then updates the clipboard and pastes.
func replaceClipboardText(hotkeyStr string) {
	// Read the current clipboard content
	origText, err := clipboard.ReadAll()
	if err != nil {
		log.Printf("Failed to read clipboard: %v", err)
		return
	}

	// Determine if this is new content or our previously transformed content
	isNewContent := lastTransformedClipboard == "" || origText != lastTransformedClipboard

	// Start with original text for transformation
	newText := origText
	totalReplacements := 0

	// Track which profiles are being used
	var activeProfiles []string

	// Apply replacements from all enabled profiles that match this hotkey
	for _, profile := range config.Profiles {
		if profile.Enabled && profile.Hotkey == hotkeyStr {
			activeProfiles = append(activeProfiles, profile.Name)
			profileReplacements := 0

			// Apply each regex replacement rule from this profile
			for _, rep := range profile.Replacements {
				re, err := regexp.Compile(rep.Regex)
				if err != nil {
					log.Printf("Invalid regex '%s' in profile '%s': %v",
						rep.Regex, profile.Name, err)
					continue
				}

				// Count matches before replacement
				matches := re.FindAllStringIndex(newText, -1)
				matchCount := 0
				if matches != nil {
					matchCount = len(matches)
				}

				// Apply replacement
				newText = re.ReplaceAllString(newText, rep.ReplaceWith)

				// Add to counters
				totalReplacements += matchCount
				profileReplacements += matchCount
			}

			log.Printf("Applied %d replacements from profile '%s'",
				profileReplacements, profile.Name)
		}
	}

	// Handle temporary clipboard storage if needed
	if config.TemporaryClipboard && (isNewContent || previousClipboard == "") {
		previousClipboard = origText
		// Enable the revert option in systray
		if miRevert != nil {
			miRevert.Enable()
		}
	}

	// Update the clipboard with the replaced text
	if err := clipboard.WriteAll(newText); err != nil {
		log.Printf("Failed to write to clipboard: %v", err)
		return
	}

	// Track what was just placed in the clipboard
	lastTransformedClipboard = newText

	// Show notification if replacements were made
	if totalReplacements > 0 {
		log.Printf("Clipboard updated with %d total replacements from profiles: %s",
			totalReplacements, strings.Join(activeProfiles, ", "))

		var message string

		if len(activeProfiles) > 1 {
			message = fmt.Sprintf("%d replacements applied from profiles: %s",
				totalReplacements, strings.Join(activeProfiles, ", "))
		} else {
			message = fmt.Sprintf("%d replacements applied from profile: %s",
				totalReplacements, activeProfiles[0])
		}

		if config.TemporaryClipboard {
			if config.AutomaticReversion {
				message += ". Clipboard will be automatically reverted after paste."
			} else {
				message += ". Original text stored for manual reversion."
			}
		}

		showNotification("Clipboard Updated", message)
	} else {
		log.Println("No regex replacements applied; no notification sent.")
	}

	// Short delay to allow clipboard update
	time.Sleep(20 * time.Millisecond)
	pasteClipboardContent()

	// Handle automatic reversion after paste if enabled
	if config.TemporaryClipboard && config.AutomaticReversion && previousClipboard != "" {
		// Give a small delay after paste to ensure the paste operation completes
		time.Sleep(50 * time.Millisecond)

		// Restore original clipboard
		if err := clipboard.WriteAll(previousClipboard); err != nil {
			log.Printf("Failed to automatically restore original clipboard: %v", err)
		} else {
			log.Println("Original clipboard content automatically restored after paste.")
		}
	}
}

// pasteClipboardContent simulates a paste action.
// On Windows, it uses the user32.dll keybd_event API.
func pasteClipboardContent() {
	switch runtime.GOOS {
	case "windows":
		keyboard := syscall.NewLazyDLL("user32.dll")
		keybd_event := keyboard.NewProc("keybd_event")
		// VK_CONTROL = 0x11, VK_V = 0x56
		keybd_event.Call(0x11, 0, 0, 0) // Press Ctrl
		keybd_event.Call(0x56, 0, 0, 0) // Press V
		keybd_event.Call(0x56, 0, 2, 0) // Release V
		keybd_event.Call(0x11, 0, 2, 0) // Release Ctrl
	case "linux":
		if err := exec.Command("xdotool", "key", "ctrl+v").Run(); err != nil {
			log.Printf("Failed to simulate paste on Linux: %v", err)
		}
	default:
		log.Println("Automatic paste not supported on this platform.")
	}
}

// restoreOriginalClipboard reverts the clipboard to its previous content.
func restoreOriginalClipboard() {
	if previousClipboard != "" {
		if err := clipboard.WriteAll(previousClipboard); err != nil {
			log.Printf("Failed to restore original clipboard: %v", err)
		} else {
			log.Println("Original clipboard content restored.")
			showNotification("Clipboard Reverted", "Original clipboard content has been restored.")
		}

		// Clear the previous clipboard and disable the revert option
		previousClipboard = ""
		if miRevert != nil {
			miRevert.Disable()
		}
	}
}

// ---------------------------------------------------------------------------
// 5. Global Hotkey Setup & Systray Menu
// ---------------------------------------------------------------------------

// parseHotkey converts a string hotkey combination (e.g., "ctrl+alt+v")
// into hotkey modifiers and key.
func parseHotkey(hotkeyStr string) ([]hotkey.Modifier, hotkey.Key, error) {
	parts := strings.Split(strings.ToLower(hotkeyStr), "+")
	var modifiers []hotkey.Modifier

	// Get the key (last part)
	keyStr := parts[len(parts)-1]
	key, exists := KeyMap[keyStr]
	if !exists {
		return nil, 0, fmt.Errorf("unsupported key: %s", keyStr)
	}

	// Parse modifiers (all parts except the last)
	for _, part := range parts[:len(parts)-1] {
		switch part {
		case "ctrl":
			modifiers = append(modifiers, hotkey.ModCtrl)
		case "alt":
			modifiers = append(modifiers, hotkey.ModAlt)
		case "shift":
			modifiers = append(modifiers, hotkey.ModShift)
		case "super", "win", "cmd":
			modifiers = append(modifiers, hotkey.ModWin)
		default:
			return nil, 0, fmt.Errorf("unsupported modifier: %s", part)
		}
	}

	return modifiers, key, nil
}

// registerHotkeys registers all hotkeys for enabled profiles
func registerHotkeys() {
	// Clean up existing hotkeys
	if registeredHotkeys != nil {
		for _, hk := range registeredHotkeys {
			hk.Unregister()
		}
	}

	// Create fresh map for tracking hotkeys
	registeredHotkeys = make(map[string]*hotkey.Hotkey)

	// Track which profiles use which hotkeys for logging
	hotkeyProfiles := make(map[string][]string)

	// Register hotkeys for all enabled profiles
	for _, profile := range config.Profiles {
		if !profile.Enabled {
			continue
		}

		// Add profile to the list for this hotkey
		hotkeyProfiles[profile.Hotkey] = append(hotkeyProfiles[profile.Hotkey], profile.Name)

		// Skip registration if this hotkey is already registered
		if _, exists := registeredHotkeys[profile.Hotkey]; exists {
			continue
		}

		// Parse and register the hotkey
		modifiers, key, err := parseHotkey(profile.Hotkey)
		if err != nil {
			log.Printf("Failed to parse hotkey '%s' for profile '%s': %v",
				profile.Hotkey, profile.Name, err)
			continue
		}

		hk := hotkey.New(modifiers, key)
		if err := hk.Register(); err != nil {
			log.Printf("Failed to register hotkey '%s' for profile '%s': %v",
				profile.Hotkey, profile.Name, err)
			continue
		}

		// Store in our tracking map
		registeredHotkeys[profile.Hotkey] = hk

		// Create the listener for this hotkey
		go func(hotkeyStr string) {
			hk := registeredHotkeys[hotkeyStr] // Capture the hotkey object
			for range hk.Keydown() {
				profileNames := strings.Join(hotkeyProfiles[hotkeyStr], ", ")
				log.Printf("Hotkey '%s' pressed. Processing clipboard using profiles: %s",
					hotkeyStr, profileNames)
				replaceClipboardText(hotkeyStr)
			}
		}(profile.Hotkey)

		log.Printf("Registered hotkey '%s' for profiles: %s",
			profile.Hotkey, strings.Join(hotkeyProfiles[profile.Hotkey], ", "))
	}
}

// isDevMode checks if the application is running in development mode via "go run"
func isDevMode() bool {
	execPath, err := os.Executable()
	if err != nil {
		return false
	}

	// Check if the executable is in a temporary directory, which indicates we're running via "go run"
	tempDir := os.TempDir()
	return strings.Contains(strings.ToLower(execPath), strings.ToLower(tempDir))
}

// restartApplication restarts the current application
func restartApplication() {
	log.Println("Restarting application...")

	// Check if we're running in development mode (go run)
	if isDevMode() {
		log.Println("Development mode detected. Instead of restarting, refreshing UI components...")

		// In development mode, we won't actually restart
		// Just unregister and re-register hotkeys, and update the menu
		for _, hk := range registeredHotkeys {
			hk.Unregister()
		}

		// Re-register hotkeys
		registerHotkeys()

		// For systray, we can't truly refresh it without restarting in dev mode
		// So we'll just inform the user
		showNotification("Dev Mode", "Menu changes will be visible after manually restarting the application")

		return
	}

	// Production mode - actually restart the application
	// Get the path to the current executable
	execPath, err := os.Executable()
	if err != nil {
		log.Printf("Error getting executable path: %v", err)
		showNotification("Error", "Failed to restart application")
		return
	}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Printf("Error getting current working directory: %v", err)
		showNotification("Error", "Failed to restart application")
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

	// Unregister hotkeys before exiting
	for _, hk := range registeredHotkeys {
		hk.Unregister()
	}

	// Start a new process with the same executable
	cmd := exec.Command(execPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = cwd // Set the working directory to the current directory

	// Start the new process
	if err := cmd.Start(); err != nil {
		log.Printf("Error starting new process: %v", err)
		showNotification("Error", "Failed to restart application")
		return
	}

	// Exit the current process
	systray.Quit()
	os.Exit(0)
}

// updateProfileMenuItems creates submenu items for each profile
func updateProfileMenuItems() {
	// Create a profiles submenu
	miProfiles := systray.AddMenuItem("Profiles", "Manage replacement profiles")

	// Add menu items for each profile
	for i := range config.Profiles {
		profile := &config.Profiles[i]

		// Create menu text
		var menuText string
		if profile.Enabled {
			menuText = "✓ " + profile.Name
		} else {
			menuText = "  " + profile.Name
		}

		// Create menu item with tooltip
		tooltip := fmt.Sprintf("Toggle profile: %s (Hotkey: %s)", profile.Name, profile.Hotkey)
		menuItem := miProfiles.AddSubMenuItem(menuText, tooltip)

		// Handle clicks
		go func(p *ProfileConfig, item *systray.MenuItem) {
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
				if err := saveConfig(); err != nil {
					log.Printf("Failed to save config after toggling profile: %v", err)
				}

				// Re-register hotkeys
				registerHotkeys()

				// Notify user
				status := map[bool]string{true: "enabled", false: "disabled"}[p.Enabled]
				showNotification("Profile Updated",
					fmt.Sprintf("Profile '%s' has been %s", p.Name, status))
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
			newProfile := ProfileConfig{
				Name:    fmt.Sprintf("New Profile %s", time.Now().Format("15:04:05")),
				Enabled: true,
				Hotkey:  "ctrl+alt+n",
				Replacements: []Replacement{
					{
						Regex:       "example",
						ReplaceWith: "replacement",
					},
				},
			}

			// Add to config
			config.Profiles = append(config.Profiles, newProfile)

			// Save config
			if err := saveConfig(); err != nil {
				log.Printf("Failed to save config after adding profile: %v", err)
			}

			// For adding profiles, we do need to restart to update the menu
			showNotification("Profile Added",
				fmt.Sprintf("New profile '%s' created. Restarting application to refresh menu.", newProfile.Name))

			// Wait a moment for notification to show before restarting
			time.Sleep(500 * time.Millisecond)
			restartApplication()
		}
	}()
}

// reloadConfig reloads the configuration from config.json
func reloadConfig() {
	log.Println("Reloading configuration...")

	// Store the original number of profiles and their names for comparison
	originalProfileCount := len(config.Profiles)
	originalProfileNames := make(map[string]bool)
	for _, profile := range config.Profiles {
		originalProfileNames[profile.Name] = true
	}

	// Store current enabled status of profiles to preserve user's runtime choices
	enabledStatus := make(map[string]bool)
	for _, profile := range config.Profiles {
		enabledStatus[profile.Name] = profile.Enabled
	}

	// Load the updated configuration
	if err := loadConfig(); err != nil {
		log.Printf("Error reloading configuration: %v", err)
		showNotification("Configuration Error",
			"Failed to reload configuration. Check logs for details.")
		return
	}

	// Restore enabled status for profiles that still exist
	// This preserves the user's runtime choices even after a config reload
	for i, profile := range config.Profiles {
		if enabled, exists := enabledStatus[profile.Name]; exists {
			config.Profiles[i].Enabled = enabled
		}
	}

	// Check if profile structure has changed
	profileStructureChanged := originalProfileCount != len(config.Profiles)

	if !profileStructureChanged {
		// Check if any profile names have changed
		for _, profile := range config.Profiles {
			if !originalProfileNames[profile.Name] {
				profileStructureChanged = true
				break
			}
		}
	}

	log.Println("Configuration reloaded successfully.")

	// Re-register hotkeys
	registerHotkeys()

	if profileStructureChanged {
		// If profile structure changed, we need to restart to rebuild the menu
		showNotification("Configuration Reloaded",
			"Profile structure has changed. Restarting application to refresh menu.")

		// Wait a moment for notification to show
		time.Sleep(500 * time.Millisecond)
		restartApplication()
	} else {
		// For simple config changes, just update in memory
		showNotification("Configuration Reloaded",
			"Configuration updated successfully. Hotkeys have been refreshed.")
	}
}

// onReady is called by systray once the tray is ready.
func onReady() {
	// Set title and tooltip
	systray.SetTitle(fmt.Sprintf("Clipboard Regex Replace %s", version))
	systray.SetTooltip(fmt.Sprintf("Clipboard Regex Replace %s", version))
	systray.SetIcon(embeddedIcon)

	// Add version info (disabled)
	miVersion := systray.AddMenuItem(fmt.Sprintf("Version: %s", version), "Clipboard Regex Replace version")
	miVersion.Disable()

	// Add profiles menu
	updateProfileMenuItems()

	// Add configuration and application options
	miReloadConfig := systray.AddMenuItem("Reload Configuration", "Reload configuration from config.json")
	miRestartApp := systray.AddMenuItem("Restart Application", "Completely restart the application to refresh menu")

	// Add clipboard revert option if enabled
	if config.TemporaryClipboard {
		miRevert = systray.AddMenuItem("Revert to Original", "Revert to original clipboard text")
		miRevert.Disable() // Disabled initially until we have an original to revert to
	}

	// Add quit option
	miQuit := systray.AddMenuItem("Quit", "Exit the application")

	// Register hotkeys
	registerHotkeys()

	// Set up menu handlers
	go func() {
		for range miReloadConfig.ClickedCh {
			reloadConfig()
		}
	}()

	go func() {
		for range miRestartApp.ClickedCh {
			restartApplication()
		}
	}()

	if config.TemporaryClipboard {
		go func() {
			for range miRevert.ClickedCh {
				restoreOriginalClipboard()
			}
		}()
	}

	go func() {
		<-miQuit.ClickedCh
		// Unregister all hotkeys
		for _, hk := range registeredHotkeys {
			hk.Unregister()
		}
		systray.Quit()
		log.Println("Exiting application.")
	}()
}

func onExit() {
	// Nothing special to do on exit
}

// ---------------------------------------------------------------------------
// 6. Main: Load configuration and run the systray.
// ---------------------------------------------------------------------------

func main() {
	log.Printf("Clipboard Regex Replace %s starting...", version)
	if err := loadConfig(); err != nil {
		log.Fatalf("Error loading config: %v", err)
	}
	systray.Run(onReady, onExit)
}
