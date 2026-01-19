package hotkey

import "errors"

// ErrBackendNotAvailable is returned when a backend cannot be used on the current system.
var ErrBackendNotAvailable = errors.New("backend not available on this system")

// Backend is an interface that abstracts different hotkey registration implementations.
// This allows us to support multiple display servers (Windows, X11, Wayland) without
// breaking existing functionality.
type Backend interface {
	// Register registers a single hotkey with the given string representation.
	// Returns a RegisteredHotkey handle and any error encountered.
	Register(hotkeyStr string) (RegisteredHotkey, error)

	// Unregister removes a previously registered hotkey.
	Unregister(hotkeyStr string) error

	// UnregisterAll removes all hotkeys registered by this backend.
	UnregisterAll() error

	// Name returns a human-readable name for this backend (for logging).
	Name() string

	// IsAvailable returns true if this backend can be used on the current system.
	IsAvailable() bool
}

// RegisteredHotkey represents a registered hotkey and provides a channel
// that receives events when the hotkey is pressed.
type RegisteredHotkey interface {
	// Keydown returns a channel that receives events when the key is pressed.
	Keydown() <-chan struct{}

	// Close cleans up resources associated with this hotkey.
	// After calling Close, the Keydown channel should not be used.
	Close() error
}
