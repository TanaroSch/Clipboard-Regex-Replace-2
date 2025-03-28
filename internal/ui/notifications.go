package ui

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gen2brain/beeep"
	"github.com/go-toast/toast"
)

// NotificationManager handles showing notifications across platforms
type NotificationManager struct {
	useNotifications bool
	appName          string
	embeddedIcon     []byte
}

// NewNotificationManager creates a new notification manager
func NewNotificationManager(useNotifications bool, appName string, embeddedIcon []byte) *NotificationManager {
	return &NotificationManager{
		useNotifications: useNotifications,
		appName:          appName,
		embeddedIcon:     embeddedIcon,
	}
}

// ShowNotification displays a desktop notification if enabled
func (n *NotificationManager) ShowNotification(title, message string) {
	if !n.useNotifications {
		return
	}
	
	if runtime.GOOS == "windows" {
		n.showWindowsNotification(title, message)
	} else {
		if err := beeep.Notify(title, message, ""); err != nil {
			log.Printf("Error showing beeep notification: %v", err)
		} else {
			log.Println("Beeep notification sent successfully.")
		}
	}
}

// showWindowsNotification displays a toast notification on Windows
func (n *NotificationManager) showWindowsNotification(title, message string) {
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
		iconPathForToast, err2 = writeTempIcon(n.embeddedIcon)
		if err2 != nil {
			log.Printf("Error writing temporary icon: %v", err2)
			iconPathForToast = "" // fallback: no icon
		} else {
			// Remove the temporary file after 10 seconds.
			time.AfterFunc(10*time.Second, func() { os.Remove(iconPathForToast) })
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
		log.Printf("Error showing toast notification: %v", err)
	} else {
		log.Println("Toast notification sent successfully.")
	}
}

// writeTempIcon writes the embedded icon to a temporary file
// and returns its absolute path. This is used as a fallback for toast notifications.
func writeTempIcon(iconData []byte) (string, error) {
	tmpFile, err := ioutil.TempFile("", "icon-*.ico")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()
	
	if _, err := tmpFile.Write(iconData); err != nil {
		return "", err
	}
	
	absPath, err := filepath.Abs(tmpFile.Name())
	if err != nil {
		return tmpFile.Name(), nil
	}
	
	return absPath, nil
}

// Global function for simplicity when detailed control isn't needed
var globalNotificationManager *NotificationManager

// InitGlobalNotifications initializes the global notification manager
func InitGlobalNotifications(useNotifications bool, appName string, embeddedIcon []byte) {
	globalNotificationManager = NewNotificationManager(useNotifications, appName, embeddedIcon)
}

// ShowNotification is a convenience function for showing notifications
// without directly referencing the notification manager
func ShowNotification(title, message string) {
	if globalNotificationManager != nil {
		globalNotificationManager.ShowNotification(title, message)
	} else {
		log.Printf("Notification not shown (disabled or manager not initialized): %s - %s", title, message)
	}
}