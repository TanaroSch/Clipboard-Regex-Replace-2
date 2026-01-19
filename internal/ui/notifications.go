package ui

import (
	"log"
	"strings"

	"github.com/TanaroSch/clipboard-regex-replace/internal/config" // Need config access
)

// NotificationLevel defines the verbosity levels for administrative notifications.
type NotificationLevel int

const (
	LevelNone  NotificationLevel = iota // 0
	LevelError                          // 1
	LevelWarn                           // 2
	LevelInfo                           // 3
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
	if err := n.platformNotify(title, message); err != nil {
		log.Printf("Error showing notification: %v", err)
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
