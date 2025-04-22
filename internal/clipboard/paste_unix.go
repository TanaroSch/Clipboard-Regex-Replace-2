//go:build !windows
// +build !windows

package clipboard

import (
	"log"
	"os/exec"
)

// simulatePlatformPaste tries to paste on Linux, macOS, etc.
// This function provides the implementation for non-Windows builds.
func simulatePlatformPaste() {
	log.Println("Attempting to paste on non-Windows platform")

	// Try xdotool first (Common on Linux X11)
	cmdXdotool := exec.Command("xdotool", "key", "ctrl+v")
	if err := cmdXdotool.Run(); err == nil {
		log.Println("Paste simulation with xdotool successful")
		return // Success
	} else {
		// Log failure, but don't return yet, try other methods
		log.Printf("xdotool paste failed (is it installed?): %v", err)
	}

	// Try wtype (Common on Linux Wayland)
	cmdWtype := exec.Command("wtype", "-M", "ctrl", "-P", "v", "-m", "ctrl")
	if err := cmdWtype.Run(); err == nil {
		log.Println("Paste simulation with wtype successful")
		return // Success
	} else {
		log.Printf("wtype paste failed (is it installed?): %v", err)
	}

	// Try osascript (macOS)
	macScript := `tell application "System Events" to keystroke "v" using command down`
	cmdOsascript := exec.Command("osascript", "-e", macScript)
	// Use Output or CombinedOutput to potentially capture errors reported by osascript itself
	if output, err := cmdOsascript.CombinedOutput(); err == nil {
		log.Println("Paste simulation with osascript successful")
		return // Success
	} else {
		log.Printf("osascript paste failed: %v\nOutput: %s", err, string(output))
	}

	// If all methods failed
	log.Println("All non-Windows paste simulation methods failed. Automatic paste might not be supported or require specific tools (xdotool, wtype, osascript).")
}