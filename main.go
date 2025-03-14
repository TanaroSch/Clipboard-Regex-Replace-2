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

const version = "v1.3.1" // Application version

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

// Config holds the application configuration loaded from config.json.
type Config struct {
	Hotkey             string        `json:"hotkey"`              // e.g. "ctrl+alt+v"
	UseNotifications   bool          `json:"use_notifications"`   // true/false
	TemporaryClipboard bool          `json:"temporary_clipboard"` // enable storing the original clipboard temporarily
	AutomaticReversion bool          `json:"automatic_reversion"` // automatically revert clipboard after pasting
	Replacements       []Replacement `json:"replacements"`
}

// Replacement represents one regex replacement rule.
type Replacement struct {
	Regex       string `json:"regex"`
	ReplaceWith string `json:"replace_with"`
}

var config Config

// loadConfig reads and parses the configuration file.
func loadConfig() error {
	data, err := ioutil.ReadFile("config.json")
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &config)
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

// Global variables for temporary clipboard handling.
var previousClipboard string

// Systray menu item for interactive action.
var miRevert *systray.MenuItem

// Store the global hotkey reference for reloading config
var hk *hotkey.Hotkey

// replaceClipboardText reads the clipboard text, applies regex replacements,
// updates the clipboard, shows a notification (if replacements occurred),
// and simulates a paste action.
// If the TemporaryClipboard option is enabled, it stores the original text
// until the user manually reverts it.
// Track clipboard transformations
var lastTransformedClipboard string // Stores the most recent text placed in clipboard after transformation

func replaceClipboardText() {
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

	// Apply each regex replacement rule
	for _, rep := range config.Replacements {
		re, err := regexp.Compile(rep.Regex)
		if err != nil {
			log.Printf("Invalid regex '%s': %v", rep.Regex, err)
			continue
		}

		// Count matches before replacement
		matches := re.FindAllStringIndex(newText, -1)
		if matches != nil {
			totalReplacements += len(matches)
		}
		newText = re.ReplaceAllString(newText, rep.ReplaceWith)
	}

	// Store original clipboard text only when:
	// 1. The user has copied new content, or
	// 2. This is the first run and previousClipboard is empty
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

	// Notify only if replacements were made
	if totalReplacements > 0 {
		log.Printf("Clipboard updated with %d replacements.", totalReplacements)
		if config.TemporaryClipboard {
			if config.AutomaticReversion {
				showNotification("Clipboard Updated",
					fmt.Sprintf("%d replacements done. Clipboard will be automatically reverted after paste.", totalReplacements))
			} else {
				showNotification("Clipboard Updated",
					fmt.Sprintf("%d replacements done. Original text stored for manual reversion via tray menu.", totalReplacements))
			}
		} else {
			showNotification("Clipboard Updated", fmt.Sprintf("%d replacements done", totalReplacements))
		}
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

// reloadConfig reloads the configuration from config.json
func reloadConfig() {
	log.Println("Reloading configuration...")
	if err := loadConfig(); err != nil {
		log.Printf("Error reloading configuration: %v", err)
		showNotification("Configuration Error", "Failed to reload configuration. Check logs for details.")
	} else {
		log.Println("Configuration reloaded successfully.")
		showNotification("Configuration Reloaded", "Configuration has been updated successfully.")

		// Unregister the existing hotkey
		if hk != nil {
			hk.Unregister()
		}

		// Register a new hotkey with the updated configuration
		modifiers, key, err := parseHotkey(config.Hotkey)
		if err != nil {
			log.Printf("Failed to parse new hotkey configuration: %v", err)
			showNotification("Hotkey Error", "Failed to register new hotkey. Check logs for details.")
		} else {
			// Register new hotkey
			hk = hotkey.New(modifiers, key)
			if err := hk.Register(); err != nil {
				log.Printf("Failed to register new hotkey: %v", err)
				showNotification("Hotkey Error", "Failed to register new hotkey. Check logs for details.")
			} else {
				log.Printf("New hotkey registered: %s", config.Hotkey)

				// Listen for hotkey events
				go func() {
					for range hk.Keydown() {
						log.Println("Hotkey pressed. Processing clipboard text...")
						replaceClipboardText()
					}
				}()
			}
		}
	}
}

// onReady is called by systray once the tray is ready.
func onReady() {
	// Set title and tooltip including version.
	systray.SetTitle(fmt.Sprintf("Clipboard Regex Replace %s", version))
	systray.SetTooltip(fmt.Sprintf("Clipboard Regex Replace %s", version))
	// Use the embedded icon for the tray.
	systray.SetIcon(embeddedIcon)

	// Add a disabled version menu item.
	miVersion := systray.AddMenuItem(fmt.Sprintf("Version: %s", version), "Clipboard Regex Replace version")
	miVersion.Disable() // Disable since it's just informational

	// Add reload configuration option
	miReloadConfig := systray.AddMenuItem("Reload Configuration", "Reload configuration from config.json")

	// If temporary clipboard functionality is enabled, add revert menu item.
	if config.TemporaryClipboard {
		miRevert = systray.AddMenuItem("Revert to Original", "Revert to original clipboard text")
		miRevert.Disable() // Disabled initially until we have an original to revert to
	}

	// Add a Quit menu item.
	mQuit := systray.AddMenuItem("Quit", "Exit the application")

	// Handle Reload Configuration clicks
	go func() {
		for range miReloadConfig.ClickedCh {
			reloadConfig()
		}
	}()

	// Handle Revert Clipboard clicks
	if config.TemporaryClipboard {
		go func() {
			for range miRevert.ClickedCh {
				restoreOriginalClipboard()
			}
		}()
	}

	// Parse the hotkey from config.
	modifiers, key, err := parseHotkey(config.Hotkey)
	if err != nil {
		log.Fatalf("Failed to parse hotkey configuration: %v", err)
	}
	// Register the hotkey using the parsed configuration.
	hk = hotkey.New(modifiers, key)
	if err := hk.Register(); err != nil {
		log.Fatalf("Failed to register hotkey: %v", err)
	}
	log.Printf("Hotkey registered: %s", config.Hotkey)

	// Listen for hotkey events.
	go func() {
		for range hk.Keydown() {
			log.Println("Hotkey pressed. Processing clipboard text...")
			replaceClipboardText()
		}
	}()

	// Quit when the tray's Quit menu item is clicked.
	go func() {
		<-mQuit.ClickedCh
		hk.Unregister()
		systray.Quit()
		log.Println("Exiting application.")
	}()
}

func onExit() {}

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
