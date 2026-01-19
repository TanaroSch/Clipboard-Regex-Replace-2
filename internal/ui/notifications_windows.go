//go:build windows

package ui

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-toast/toast"
)

func (n *NotificationManager) platformNotify(title, message string) error {
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
		// Check if embedded icon data exists before trying to write it
		if len(n.embeddedIcon) > 0 {
			log.Println("icon.png not found; using fallback embedded icon.")
			var err2 error
			iconPathForToast, err2 = writeTempIcon(n.embeddedIcon)
			if err2 != nil {
				log.Printf("Error writing temporary icon: %v", err2)
				iconPathForToast = "" // fallback: no icon
			} else {
				// Remove the temporary file after a short delay.
				time.AfterFunc(10*time.Second, func() {
					if errRem := os.Remove(iconPathForToast); errRem != nil && !os.IsNotExist(errRem) {
						log.Printf("Error removing temporary icon file %s: %v", iconPathForToast, errRem)
					}
				})
			}
		} else {
			log.Println("icon.png not found and no embedded icon available.")
			iconPathForToast = "" // No icon available
		}
	}

	notification := toast.Notification{
		AppID:   n.appName,
		Title:   title,
		Message: message,
		Icon:    iconPathForToast,
	}

	err := notification.Push()
	if err != nil {
		// Check for common toast error (e.g., notifications disabled system-wide)
		if strings.Contains(err.Error(), "notification platform is unavailable") {
			log.Println("Toast notification failed: Platform unavailable (Notifications might be disabled in Windows Settings).")
			return err
		}
		log.Printf("Error showing toast notification: %v", err)
		return err
	}

	log.Println("Toast notification sent successfully.")
	return nil
}

func writeTempIcon(iconData []byte) (string, error) {
	if len(iconData) == 0 {
		return "", fmt.Errorf("cannot write empty icon data")
	}
	tmpFile, err := os.CreateTemp("", "clipregex-icon-*.ico")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write(iconData); err != nil {
		_ = os.Remove(tmpFile.Name())
		return "", err
	}

	absPath, err := filepath.Abs(tmpFile.Name())
	if err != nil {
		log.Printf("Warning: Could not get absolute path for temp icon '%s': %v", tmpFile.Name(), err)
		return tmpFile.Name(), nil
	}

	return absPath, nil
}
