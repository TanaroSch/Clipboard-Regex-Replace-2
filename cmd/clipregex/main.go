package main

import (
	"fmt"
	"log"
	"os"
	"runtime/debug" // Import for stack trace

	"github.com/TanaroSch/clipboard-regex-replace/internal/app"
	"github.com/TanaroSch/clipboard-regex-replace/internal/config"
	"github.com/TanaroSch/clipboard-regex-replace/internal/ui" // Needed for potential panic notification
)

const version = "v1.7.0" // Bump version for secret management feature

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
		ui.InitGlobalNotifications(true, config.DefaultKeyringService, nil) // Minimal init
		ui.ShowNotification("Startup Error", errMsg)
		// Now exit fatally
		os.Exit(1)
	}

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
			ui.ShowNotification("Application Error", "A critical error occurred. Please check logs.")

			// Consider exiting with non-zero status
			os.Exit(1)
		}
	}()

	// Run the application (blocking call, likely systray.Run)
	log.Println("Starting application main loop...")
	application.Run()
	log.Println("Application main loop finished.") // This might not be reached if systray exits process
}