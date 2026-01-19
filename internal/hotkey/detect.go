package hotkey

import (
	"log"
	"os"
	"runtime"
)

// DisplayServer represents the type of display server in use
type DisplayServer int

const (
	DisplayServerUnknown DisplayServer = iota
	DisplayServerWindows
	DisplayServerX11
	DisplayServerWayland
)

func (ds DisplayServer) String() string {
	switch ds {
	case DisplayServerWindows:
		return "Windows"
	case DisplayServerX11:
		return "X11"
	case DisplayServerWayland:
		return "Wayland"
	default:
		return "Unknown"
	}
}

// DetectDisplayServer determines which display server is currently in use.
// This function is safe to call on any platform.
func DetectDisplayServer() DisplayServer {
	// Windows always uses its own system
	if runtime.GOOS == "windows" {
		log.Println("Detected display server: Windows")
		return DisplayServerWindows
	}

	// On Unix-like systems, check environment variables
	// Check Wayland first (more specific)
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		log.Println("Detected display server: Wayland (WAYLAND_DISPLAY set)")
		return DisplayServerWayland
	}

	// Check for X11
	if os.Getenv("DISPLAY") != "" {
		log.Println("Detected display server: X11 (DISPLAY set)")
		return DisplayServerX11
	}

	// macOS uses its own system, but we treat it as X11-compatible
	// because golang.design/x/hotkey supports it
	if runtime.GOOS == "darwin" {
		log.Println("Detected display server: macOS (treated as X11-compatible)")
		return DisplayServerX11
	}

	log.Println("Warning: Could not detect display server type")
	return DisplayServerUnknown
}

// HasPortalSupport checks if XDG Desktop Portal is available on the system.
// This is used to determine if Wayland global shortcuts can be supported.
func HasPortalSupport() bool {
	// Only relevant on Linux
	if runtime.GOOS != "linux" {
		return false
	}

	// Check if D-Bus session bus is available
	sessionBus := os.Getenv("DBUS_SESSION_BUS_ADDRESS")
	if sessionBus == "" {
		log.Println("D-Bus session bus not available (DBUS_SESSION_BUS_ADDRESS not set)")
		return false
	}

	// TODO: In future, we could check if the portal service is actually running
	// by attempting to connect to org.freedesktop.portal.Desktop
	// For now, we assume if D-Bus is available, portal might be available

	log.Println("D-Bus session bus detected, XDG Portal may be available")
	return true
}
