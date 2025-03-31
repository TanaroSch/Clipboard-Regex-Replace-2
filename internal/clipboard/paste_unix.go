//go:build !windows
// +build !windows

package clipboard

import (
	"log"
	"os/exec"
)

// UnixPaste tries to paste on Linux, macOS, etc.
func UnixPaste() {
	log.Println("Attempting to paste on non-Windows platform")
	
	// Try xdotool first (Linux)
	if err := exec.Command("xdotool", "key", "ctrl+v").Run(); err == nil {
		log.Println("Paste with xdotool successful")
		return
	}
	
	// Try osascript (macOS)
	macScript := `tell application "System Events" to keystroke "v" using command down`
	if err := exec.Command("osascript", "-e", macScript).Run(); err == nil {
		log.Println("Paste with osascript successful")
		return
	}
	
	log.Println("Automatic paste not supported on this platform")
}