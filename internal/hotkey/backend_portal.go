//go:build linux
// +build linux

package hotkey

import (
	"fmt"
	"log"
)

// PortalBackend uses XDG Desktop Portal GlobalShortcuts for Wayland support.
// This backend is currently a STUB and not yet implemented.
// When implemented, it will support global shortcuts on Wayland compositors
// that implement the org.freedesktop.portal.GlobalShortcuts interface.
//
// Implementation TODO:
// 1. Add github.com/godbus/dbus/v5 dependency
// 2. Connect to org.freedesktop.portal.Desktop D-Bus service
// 3. Implement CreateSession method
// 4. Implement BindShortcuts method (shows permission dialog to user)
// 5. Listen for Activated/Deactivated signals
// 6. Map signals to Keydown events
//
// References:
// - https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.GlobalShortcuts.html
// - https://github.com/godbus/dbus
type PortalBackend struct {
	// TODO: Add D-Bus connection and session tracking
	// conn    *dbus.Conn
	// session dbus.ObjectPath
	// shortcuts map[string]*portalHotkey
}

// NewPortalBackend creates a new Portal backend for Wayland.
// Currently returns a stub implementation.
func NewPortalBackend() *PortalBackend {
	log.Println("Portal backend: Creating stub backend (NOT YET IMPLEMENTED)")
	return &PortalBackend{}
}

// Name returns the name of this backend.
func (b *PortalBackend) Name() string {
	return "XDG Desktop Portal (Wayland) [STUB - NOT IMPLEMENTED]"
}

// IsAvailable checks if the Portal backend can be used.
// Currently always returns false since it's not implemented.
func (b *PortalBackend) IsAvailable() bool {
	// TODO: Check if:
	// 1. Running on Wayland (WAYLAND_DISPLAY set)
	// 2. D-Bus session bus is available
	// 3. org.freedesktop.portal.Desktop service is running
	// 4. GlobalShortcuts interface is available

	log.Println("Portal backend: Stub implementation always returns unavailable")
	return false
}

// Register is a stub that returns an error.
func (b *PortalBackend) Register(hotkeyStr string) (RegisteredHotkey, error) {
	return nil, fmt.Errorf("Portal backend not yet implemented - cannot register hotkey '%s'", hotkeyStr)
}

// Unregister is a stub that returns nil.
func (b *PortalBackend) Unregister(hotkeyStr string) error {
	log.Printf("Portal backend stub: Unregister called for '%s' (no-op)", hotkeyStr)
	return nil
}

// UnregisterAll is a stub that returns nil.
func (b *PortalBackend) UnregisterAll() error {
	log.Println("Portal backend stub: UnregisterAll called (no-op)")
	return nil
}

// Future implementation notes:
//
// type portalHotkey struct {
//     hotkeyStr string
//     keydownCh chan struct{}
//     // D-Bus signal subscription handle
// }
//
// func (ph *portalHotkey) Keydown() <-chan struct{} {
//     return ph.keydownCh
// }
//
// func (ph *portalHotkey) Close() error {
//     // Unsubscribe from D-Bus signals
//     // Close channel
//     return nil
// }
