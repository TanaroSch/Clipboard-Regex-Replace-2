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

const version = "v1.0.0" // Application version

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
	Hotkey           string        `json:"hotkey"`            // e.g. "ctrl+alt+v"
	UseNotifications bool          `json:"use_notifications"` // true/false
	Replacements     []Replacement `json:"replacements"`
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
			// Get absolute path.
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
			AppID:   "Clipboard Regex Replace", // Make sure this matches a registered AppUserModelID if needed.
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
// 4. Clipboard Processing & Paste Simulation
// ---------------------------------------------------------------------------

// replaceClipboardText reads the clipboard text, applies regex replacements,
// updates the clipboard, shows a notification (if replacements occurred),
// and simulates a paste action.
func replaceClipboardText() {
	origText, err := clipboard.ReadAll()
	if err != nil {
		log.Printf("Failed to read clipboard: %v", err)
		return
	}

	newText := origText
	totalReplacements := 0

	// Apply each regex replacement rule.
	for _, rep := range config.Replacements {
		re, err := regexp.Compile(rep.Regex)
		if err != nil {
			log.Printf("Invalid regex '%s': %v", rep.Regex, err)
			continue
		}

		// Count matches before replacement.
		matches := re.FindAllStringIndex(newText, -1)
		if matches != nil {
			totalReplacements += len(matches)
		}
		newText = re.ReplaceAllString(newText, rep.ReplaceWith)
	}

	// Update the clipboard.
	if err := clipboard.WriteAll(newText); err != nil {
		log.Printf("Failed to write to clipboard: %v", err)
		return
	}

	// Notify only if replacements were made.
	if totalReplacements > 0 {
		log.Printf("Clipboard updated with %d replacements.", totalReplacements)
		showNotification("Clipboard Updated", fmt.Sprintf("%d replacements done", totalReplacements))
	} else {
		log.Println("No regex replacements applied; no notification sent.")
	}

	// Short delay to allow clipboard update.
	time.Sleep(200 * time.Millisecond)
	pasteClipboardContent()
}

// pasteClipboardContent simulates a paste action.
// On Windows, it uses a PowerShell command with a hidden window.
func pasteClipboardContent() {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("powershell", "-command", "Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.SendKeys]::SendWait('^v')")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if err := cmd.Run(); err != nil {
			log.Printf("Failed to simulate paste on Windows: %v", err)
		}
	case "linux":
		if err := exec.Command("xdotool", "key", "ctrl+v").Run(); err != nil {
			log.Printf("Failed to simulate paste on Linux: %v", err)
		}
	default:
		log.Println("Automatic paste not supported on this platform.")
	}
}

// ---------------------------------------------------------------------------
// 5. Systray & Global Hotkey Setup
// ---------------------------------------------------------------------------

// onReady is called by systray once the tray is ready.
func onReady() {
	// Set title and tooltip including version.
	systray.SetTitle(fmt.Sprintf("Clipboard Regex Replace %s", version))
	systray.SetTooltip(fmt.Sprintf("Clipboard Regex Replace %s", version))
	// Use the embedded icon for the tray.
	systray.SetIcon(embeddedIcon)

	// Add a disabled version menu item.
	miVersion := systray.AddMenuItem(fmt.Sprintf("Version: %s", version), "Clipboard Regex Replace version")
	// We don't need to listen for clicks on the version item.
	// (It serves as an informational label.)
	go func() {
		for {
			<-miVersion.ClickedCh
		}
	}()

	// Add a Quit menu item.
	mQuit := systray.AddMenuItem("Quit", "Exit the application")

	// Register the global hotkey (assumed "ctrl+alt+v").
	hk := hotkey.New([]hotkey.Modifier{hotkey.ModCtrl, hotkey.ModAlt}, hotkey.KeyV)
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
