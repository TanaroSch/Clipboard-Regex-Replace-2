package main

import (
	"fmt"
	"log"
	"os"

	"github.com/TanaroSch/clipboard-regex-replace/internal/app"
	"github.com/TanaroSch/clipboard-regex-replace/internal/config"
)

const version = "v1.5.2"

func main() {
	log.Printf("Clipboard Regex Replace %s starting...", version)

	// Load configuration
	cfg, err := config.Load("config.json")
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Create and run the application
	application := app.New(cfg, version)
	
	// Handle any panics during execution
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Fatal error: %v\n", r)
			os.Exit(1)
		}
	}()

	// Run the application
	application.Run()
}