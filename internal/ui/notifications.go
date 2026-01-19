package ui

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/TanaroSch/clipboard-regex-replace/internal/config" // Need config access
	"github.com/gen2brain/beeep"
	"github.com/go-toast/toast"
)

// NotificationLevel defines the verbosity levels for administrative notifications.
type NotificationLevel int

const (
	LevelNone NotificationLevel = iota // 0
	LevelError                         // 1
	LevelWarn                          // 2
	LevelInfo                          // 3
)

// NotificationManager handles showing notifications across platforms
type NotificationManager struct {
	config       *config.Config // Store config reference
	appName      string
	embeddedIcon []byte
}

// NewNotificationManager creates a new notification manager
func NewNotificationManager(cfg *config.Config, appName string, embeddedIcon []byte) *NotificationManager {
	return &NotificationManager{
		config:       cfg, // Store the config
		appName:      appName,
		embeddedIcon: embeddedIcon,
	}
}

// getConfiguredAdminLevel parses the level string from config and returns the enum value.
func (n *NotificationManager) getConfiguredAdminLevel() NotificationLevel {
	if n.config == nil {
		return LevelWarn // Default if config is somehow nil
	}
	levelStr := strings.ToLower(strings.TrimSpace(n.config.AdminNotificationLevel))
	switch levelStr {
	case "none":
		return LevelNone
	case "error":
		return LevelError
	case "warn":
		return LevelWarn
	case "info":
		return LevelInfo
	default:
		log.Printf("Warning: Invalid AdminNotificationLevel '%s' in config. Defaulting to 'Warn'.", n.config.AdminNotificationLevel)
		return LevelWarn // Default for invalid values
	}
}

// showPlatformNotification handles the OS-specific notification logic.
func (n *NotificationManager) showPlatformNotification(title, message string) {
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

// ShowAdminNotification displays an administrative notification if the configured level allows it.
func (n *NotificationManager) ShowAdminNotification(requiredLevel NotificationLevel, title, message string) {
	configuredLevel := n.getConfiguredAdminLevel()

	if configuredLevel >= requiredLevel && requiredLevel != LevelNone {
		log.Printf("Showing Admin Notification (Level: %v >= Required: %v): %s - %s", configuredLevel, requiredLevel, title, message)
		n.showPlatformNotification(title, message)
	} else {
		log.Printf("Admin Notification suppressed by level (Level: %v < Required: %v): %s - %s", configuredLevel, requiredLevel, title, message)
	}
}

// ShowReplacementNotification displays a notification after a clipboard replacement, if enabled.
func (n *NotificationManager) ShowReplacementNotification(title, message string) {
	if n.config == nil || !n.config.NotifyOnReplacement {
		log.Printf("Replacement Notification suppressed by config: %s - %s", title, message)
		return
	}
	log.Printf("Showing Replacement Notification: %s - %s", title, message)
	n.showPlatformNotification(title, message)
}

// showWindowsNotification displays a toast notification on Windows (remains mostly the same)
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
		// Check if embedded icon data exists before trying to write it
		if len(n.embeddedIcon) > 0 {
			log.Println("icon.png not found; using fallback embedded icon.")
			var err2 error
			iconPathForToast, err2 = writeTempIcon(n.embeddedIcon)
			if err2 != nil {
				log.Printf("Error writing temporary icon: %v", err2)
				iconPathForToast = "" // fallback: no icon
			} else {
				// Remove the temporary file after 10 seconds.
				// Consider making this duration configurable or managing temp files better.
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
		AppID:   n.appName, // Use AppName from manager
		Title:   title,
		Message: message,
		Icon:    iconPathForToast,
	}

	err := notification.Push()
	if err != nil {
		// Check for common toast error (e.g., notifications disabled system-wide)
		// Example: "Exception: The notification platform is unavailable."
		if strings.Contains(err.Error(), "notification platform is unavailable") {
			log.Println("Toast notification failed: Platform unavailable (Notifications might be disabled in Windows Settings).")
		} else {
			log.Printf("Error showing toast notification: %v", err)
		}
	} else {
		log.Println("Toast notification sent successfully.")
	}
}

// writeTempIcon writes the embedded icon to a temporary file (remains the same)
func writeTempIcon(iconData []byte) (string, error) {
	if len(iconData) == 0 {
		return "", fmt.Errorf("cannot write empty icon data")
	}
	tmpFile, err := os.CreateTemp("", "clipregex-icon-*.ico")
	if err != nil {
		return "", err
	}
	// Close immediately after writing to ensure data is flushed
	// and the file can potentially be deleted later.
	defer tmpFile.Close()

	if _, err := tmpFile.Write(iconData); err != nil {
		// Attempt to remove the partially written file on error
		os.Remove(tmpFile.Name())
		return "", err
	}

	absPath, err := filepath.Abs(tmpFile.Name())
	if err != nil {
		// Return non-absolute path if Abs fails, but log it
		log.Printf("Warning: Could not get absolute path for temp icon '%s': %v", tmpFile.Name(), err)
		return tmpFile.Name(), nil
	}

	return absPath, nil
}

// --- Global Access ---

var globalNotificationManager *NotificationManager

// InitGlobalNotifications initializes the global notification manager
func InitGlobalNotifications(cfg *config.Config, appName string, embeddedIcon []byte) {
	globalNotificationManager = NewNotificationManager(cfg, appName, embeddedIcon)
	log.Printf("Global Notification Manager Initialized (Admin Level: %s, Replacement Notify: %t)",
		cfg.AdminNotificationLevel, cfg.NotifyOnReplacement)
}

// ShowAdminNotification is a convenience function for showing administrative notifications
func ShowAdminNotification(requiredLevel NotificationLevel, title, message string) {
	if globalNotificationManager != nil {
		globalNotificationManager.ShowAdminNotification(requiredLevel, title, message)
	} else {
		log.Printf("Admin Notification not shown (manager not initialized): %s - %s", title, message)
	}
}

// ShowReplacementNotification is a convenience function for showing replacement notifications
func ShowReplacementNotification(title, message string) {
	if globalNotificationManager != nil {
		globalNotificationManager.ShowReplacementNotification(title, message)
	} else {
		log.Printf("Replacement Notification not shown (manager not initialized): %s - %s", title, message)
	}
}