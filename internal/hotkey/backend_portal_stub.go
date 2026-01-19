//go:build !linux
// +build !linux

package hotkey

import "log"

// PortalBackend stub for non-Linux platforms.
// This ensures the code compiles on Windows and macOS even though
// the Portal backend is Linux-specific.
type PortalBackend struct{}

// NewPortalBackend creates a stub that's never used on non-Linux platforms.
func NewPortalBackend() *PortalBackend {
	log.Println("Portal backend: Not available on non-Linux platforms")
	return &PortalBackend{}
}

// Name returns the name of this backend.
func (b *PortalBackend) Name() string {
	return "XDG Desktop Portal (Linux only)"
}

// IsAvailable always returns false on non-Linux platforms.
func (b *PortalBackend) IsAvailable() bool {
	return false
}

// Register always returns an error on non-Linux platforms.
func (b *PortalBackend) Register(hotkeyStr string) (RegisteredHotkey, error) {
	return nil, ErrBackendNotAvailable
}

// Unregister is a no-op on non-Linux platforms.
func (b *PortalBackend) Unregister(hotkeyStr string) error {
	return nil
}

// UnregisterAll is a no-op on non-Linux platforms.
func (b *PortalBackend) UnregisterAll() error {
	return nil
}
