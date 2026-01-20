package main

import (
	"fmt"
	"log"
	"os"
	"runtime/debug" // Import for stack trace

	"github.com/TanaroSch/clipboard-regex-replace/internal/app"
	"github.com/TanaroSch/clipboard-regex-replace/internal/config"
	"github.com/TanaroSch/clipboard-regex-replace/internal/ui"
	"github.com/TanaroSch/clipboard-regex-replace/internal/resources"
)

const version = "v1.8.1"

func main() {
	// Configure logging maybe? (e.g., write to file)
	// log.SetOutput(...)

	log.Printf("Clipboard Regex Replace %s starting...", version)

	// Attempt to create default config if needed BEFORE loading
	if err := config.CreateDefaultConfig("config.json"); err != nil {
		// Log warning, but continue trying to load, as it might exist anyway
		log.Printf("Warning: Failed to create default config (it might already exist or dir is not writable): %v", err)
	}

	// Load configuration (this now includes loading secrets from keyring)
	cfg, err := config.Load("config.json")
	if err != nil {
		// Provide more context if it's a keyring issue maybe? Difficult to tell generically.
		errMsg := fmt.Sprintf("FATAL: Error loading config/secrets: %v. Check config.json and OS keychain/credential manager access.", err)
		log.Print(errMsg) // Use Println or Printf, not Fatalf yet

		// Try to show a notification before exiting? Only if UI is somewhat initializable
		// Attempt to initialize with temporary minimal config for notification
		tempCfgForNotify := &config.Config{
			AdminNotificationLevel: config.DefaultAdminNotificationLevel, // Use default level for startup errors
			NotifyOnReplacement:    false,                               // Not relevant here
		}
		ui.InitGlobalNotifications(tempCfgForNotify, config.DefaultKeyringService, nil) // Minimal init
		// Use the specific notification function with Error level
		ui.ShowAdminNotification(ui.LevelError, "Startup Error", errMsg) // <<< CHANGED
		// Now exit fatally
		os.Exit(1)
	}

	// Initialize Notifications fully AFTER config is loaded successfully
	// Retrieve icon data (error handled within New)
	appIcon, iconErr := resources.GetIcon() // Call the function from resources package
	if iconErr != nil {
		log.Printf("Warning: Failed to load application icon: %v", iconErr)
		// appIcon will be nil or empty, InitGlobalNotifications should handle this
	}
	ui.InitGlobalNotifications(cfg, config.DefaultKeyringService, appIcon)

	// Create and run the application
	application := app.New(cfg, version)

	// Handle any panics during execution
	defer func() {
		if r := recover(); r != nil {
			// Log the panic and stack trace
			stackTrace := string(debug.Stack())
			errMsg := fmt.Sprintf("FATAL PANIC: %v\n%s", r, stackTrace)
			log.Print(errMsg)
			// Also print to stderr for console visibility
			fmt.Fprintf(os.Stderr, "%s\n", errMsg)

			// Attempt to show a UI notification about the crash
			// UI might not be fully running, but worth a try
			ui.ShowAdminNotification(ui.LevelError, "Application Error", "A critical error occurred. Please check logs.") // <<< CHANGED

			// Consider exiting with non-zero status
			os.Exit(1)
		}
	}()

	// Run the application (blocking call, likely systray.Run)
	log.Println("Starting application main loop...")
	application.Run()
	log.Println("Application main loop finished.") // This might not be reached if systray exits process
}