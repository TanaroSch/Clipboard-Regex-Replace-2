package hotkey

import (
	"fmt"
	"log"
	"runtime"
	"sync"

	"golang.design/x/hotkey"
)

// LegacyBackend wraps the existing golang.design/x/hotkey library.
// This backend supports Windows, macOS, and X11 on Linux.
// It does NOT support Wayland.
type LegacyBackend struct {
	mu               sync.RWMutex
	registeredKeys   map[string]*legacyHotkey
	displayServer    DisplayServer
}

// NewLegacyBackend creates a new legacy backend using golang.design/x/hotkey.
func NewLegacyBackend() *LegacyBackend {
	ds := DetectDisplayServer()
	log.Printf("Legacy backend: Detected display server: %s", ds)

	return &LegacyBackend{
		registeredKeys: make(map[string]*legacyHotkey),
		displayServer:  ds,
	}
}

// Name returns the name of this backend.
func (b *LegacyBackend) Name() string {
	return "Legacy (golang.design/x/hotkey)"
}

// IsAvailable checks if this backend can be used on the current system.
func (b *LegacyBackend) IsAvailable() bool {
	// Legacy backend works on Windows, macOS, and X11
	switch b.displayServer {
	case DisplayServerWindows:
		return true
	case DisplayServerX11:
		return true
	case DisplayServerWayland:
		// golang.design/x/hotkey does NOT support Wayland
		log.Println("Legacy backend: Not available on Wayland")
		return false
	default:
		log.Println("Legacy backend: Unknown display server, assuming unavailable")
		return false
	}
}

// Register registers a hotkey using the legacy backend.
func (b *LegacyBackend) Register(hotkeyStr string) (RegisteredHotkey, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Check if already registered
	if existing, exists := b.registeredKeys[hotkeyStr]; exists {
		log.Printf("Legacy backend: Hotkey '%s' already registered, returning existing", hotkeyStr)
		return existing, nil
	}

	// Parse the hotkey string
	modifiers, key, err := parseHotkey(hotkeyStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse hotkey '%s': %w", hotkeyStr, err)
	}

	// Create and register the hotkey using the existing library
	hk := hotkey.New(modifiers, key)
	if err := hk.Register(); err != nil {
		return nil, fmt.Errorf("failed to register hotkey '%s': %w", hotkeyStr, err)
	}

	// Wrap in our interface
	wrapped := &legacyHotkey{
		hotkey:    hk,
		hotkeyStr: hotkeyStr,
		keydownCh: make(chan struct{}),
		stopCh:    make(chan struct{}),
	}

	// Start the event converter goroutine
	wrapped.startEventConverter()

	b.registeredKeys[hotkeyStr] = wrapped
	log.Printf("Legacy backend: Successfully registered hotkey '%s'", hotkeyStr)

	return wrapped, nil
}

// Unregister removes a single hotkey.
func (b *LegacyBackend) Unregister(hotkeyStr string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	hk, exists := b.registeredKeys[hotkeyStr]
	if !exists {
		log.Printf("Legacy backend: Hotkey '%s' not found for unregister", hotkeyStr)
		return nil
	}

	if err := hk.Close(); err != nil {
		log.Printf("Legacy backend: Error unregistering '%s': %v", hotkeyStr, err)
		return err
	}

	delete(b.registeredKeys, hotkeyStr)
	log.Printf("Legacy backend: Unregistered hotkey '%s'", hotkeyStr)
	return nil
}

// UnregisterAll removes all registered hotkeys.
func (b *LegacyBackend) UnregisterAll() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	log.Printf("Legacy backend: Unregistering all %d hotkeys", len(b.registeredKeys))

	for hotkeyStr, hk := range b.registeredKeys {
		if err := hk.Close(); err != nil {
			log.Printf("Legacy backend: Error unregistering '%s': %v", hotkeyStr, err)
		}
	}

	b.registeredKeys = make(map[string]*legacyHotkey)
	return nil
}

// legacyHotkey wraps golang.design/x/hotkey.Hotkey to implement RegisteredHotkey interface.
type legacyHotkey struct {
	hotkey    *hotkey.Hotkey
	hotkeyStr string
	keydownCh chan struct{} // Converted channel for interface compatibility
	stopCh    chan struct{} // Signal to stop the converter goroutine
}

// Keydown returns the channel that receives keydown events.
func (lh *legacyHotkey) Keydown() <-chan struct{} {
	return lh.keydownCh
}

// startEventConverter converts hotkey.Event channel to struct{} channel.
// This bridges the golang.design/x/hotkey API with our Backend interface.
func (lh *legacyHotkey) startEventConverter() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("RECOVERED FROM PANIC IN LEGACY HOTKEY CONVERTER (%s): %v", lh.hotkeyStr, r)
			}
		}()

		for {
			select {
			case <-lh.stopCh:
				close(lh.keydownCh)
				return
			case <-lh.hotkey.Keydown():
				// Convert Event to struct{} by just signaling on our channel
				select {
				case lh.keydownCh <- struct{}{}:
				case <-lh.stopCh:
					close(lh.keydownCh)
					return
				}
			}
		}
	}()
}

// Close unregisters the hotkey and cleans up resources.
func (lh *legacyHotkey) Close() error {
	if lh.hotkey == nil {
		return nil
	}

	// Signal the converter goroutine to stop
	close(lh.stopCh)

	// Unregister the hotkey
	if err := lh.hotkey.Unregister(); err != nil {
		return fmt.Errorf("failed to unregister hotkey '%s': %w", lh.hotkeyStr, err)
	}

	return nil
}

// SelectBackend chooses the appropriate backend based on the current environment.
// This function prioritizes compatibility and graceful degradation:
// 1. Windows/X11/macOS: Use LegacyBackend (existing golang.design/x/hotkey)
// 2. Wayland with Portal: Use PortalBackend (future implementation)
// 3. Wayland without Portal: Return nil (no hotkey support)
func SelectBackend() Backend {
	ds := DetectDisplayServer()

	switch ds {
	case DisplayServerWindows, DisplayServerX11:
		// Use the proven legacy backend for Windows and X11
		backend := NewLegacyBackend()
		if backend.IsAvailable() {
			log.Printf("Selected backend: %s for %s", backend.Name(), ds)
			return backend
		}
		log.Printf("Warning: Legacy backend not available for %s", ds)
		return nil

	case DisplayServerWayland:
		// Try Portal backend first (when implemented)
		// For now, check if Portal is available and log
		if HasPortalSupport() {
			log.Println("Wayland detected with potential Portal support")
			log.Println("Portal backend not yet implemented - hotkeys will be disabled")
			// TODO: return NewPortalBackend() when implemented
			return nil
		}

		// No Portal support on Wayland means no hotkeys
		log.Println("Wayland detected without Portal support - hotkeys unavailable")
		log.Println("Clipboard operations will still work, but hotkeys are disabled")
		return nil

	default:
		log.Printf("Warning: Unknown display server, hotkeys unavailable")
		return nil
	}
}

// Compatibility check: Ensure this compiles on all platforms
var _ = runtime.GOOS // Use runtime to avoid "imported and not used" error
