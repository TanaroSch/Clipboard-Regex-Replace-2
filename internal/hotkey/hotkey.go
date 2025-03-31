package hotkey

import (
	"fmt"
	"log"
	"strings"

	"github.com/TanaroSch/clipboard-regex-replace/internal/config"
	"golang.design/x/hotkey"
)

// Manager handles registration and lifecycle of global hotkeys
type Manager struct {
	config            *config.Config
	registeredHotkeys map[string]*hotkey.Hotkey
	onTrigger         func(string, bool) // hotkeyStr, isReverse
	onRevert          func()
}

// NewManager creates a new hotkey manager
func NewManager(cfg *config.Config, onTrigger func(string, bool), onRevert func()) *Manager {
	return &Manager{
		config:            cfg,
		registeredHotkeys: make(map[string]*hotkey.Hotkey),
		onTrigger:         onTrigger,
		onRevert:          onRevert,
	}
}

// RegisterAll registers all hotkeys for enabled profiles
func (m *Manager) RegisterAll() error {
	// Clean up existing hotkeys
	m.UnregisterAll()

	// Track which profiles use which hotkeys for logging
	hotkeyProfiles := make(map[string][]string)

	// Register hotkeys for all enabled profiles
	for _, profile := range m.config.Profiles {
		if !profile.Enabled {
			continue
		}

		// Add profile to the list for this hotkey
		hotkeyProfiles[profile.Hotkey] = append(hotkeyProfiles[profile.Hotkey], profile.Name)

		// Add to reverse hotkey tracking if it exists
		if profile.ReverseHotkey != "" {
			hotkeyProfiles[profile.ReverseHotkey] = append(
				hotkeyProfiles[profile.ReverseHotkey], profile.Name+" (reverse)")
		}

		// Register standard hotkey
		if err := m.registerProfileHotkey(profile, profile.Hotkey, false); err != nil {
			return fmt.Errorf("failed to register hotkey '%s' for profile '%s': %v", 
				profile.Hotkey, profile.Name, err)
		}

		// Register reverse hotkey if specified
		if profile.ReverseHotkey != "" {
			if err := m.registerProfileHotkey(profile, profile.ReverseHotkey, true); err != nil {
				return fmt.Errorf("failed to register reverse hotkey '%s' for profile '%s': %v", 
					profile.ReverseHotkey, profile.Name, err)
			}
		}
	}

	// Register the global revert hotkey if configured and applicable
	if m.config.RevertHotkey != "" && m.config.TemporaryClipboard && !m.config.AutomaticReversion {
		if err := m.registerRevertHotkey(m.config.RevertHotkey); err != nil {
			return fmt.Errorf("failed to register revert hotkey '%s': %v", 
				m.config.RevertHotkey, err)
		}
	}

	return nil
}

// UnregisterAll unregisters all currently registered hotkeys
func (m *Manager) UnregisterAll() {
	for _, hk := range m.registeredHotkeys {
		hk.Unregister()
	}
	m.registeredHotkeys = make(map[string]*hotkey.Hotkey)
}

// registerProfileHotkey registers a hotkey for a profile
func (m *Manager) registerProfileHotkey(profile config.ProfileConfig, hotkeyStr string, isReverse bool) error {
	// Skip if already registered
	if _, exists := m.registeredHotkeys[hotkeyStr]; exists {
		return nil
	}

	// Parse and register the hotkey
	modifiers, key, err := parseHotkey(hotkeyStr)
	if err != nil {
		return err
	}

	hk := hotkey.New(modifiers, key)
	if err := hk.Register(); err != nil {
		return err
	}

	// Store in our tracking map
	m.registeredHotkeys[hotkeyStr] = hk

	// Direction suffix for logging
	directionSuffix := ""
	if isReverse {
		directionSuffix = " (reverse)"
	}

	// Create the listener for this hotkey
	go func(hotkeyStr string, isReverse bool) {
		hk := m.registeredHotkeys[hotkeyStr] // Capture the hotkey object
		for range hk.Keydown() {
			log.Printf("Hotkey '%s' pressed. Processing clipboard using profile: %s%s",
				hotkeyStr, profile.Name, directionSuffix)

			// Call the callback function
			if m.onTrigger != nil {
				m.onTrigger(hotkeyStr, isReverse)
			}
		}
	}(hotkeyStr, isReverse)

	log.Printf("Registered hotkey '%s' for profile: %s%s",
		hotkeyStr, profile.Name, directionSuffix)

	return nil
}

// registerRevertHotkey registers a global hotkey for reverting the clipboard
func (m *Manager) registerRevertHotkey(hotkeyStr string) error {
	// Skip if already registered
	if _, exists := m.registeredHotkeys[hotkeyStr]; exists {
		return nil
	}

	// Parse the hotkey
	modifiers, key, err := parseHotkey(hotkeyStr)
	if err != nil {
		return err
	}

	// Register the hotkey
	hk := hotkey.New(modifiers, key)
	if err := hk.Register(); err != nil {
		return err
	}

	// Store in our tracking map
	m.registeredHotkeys[hotkeyStr] = hk

	// Create the listener for this hotkey
	go func() {
		for range hk.Keydown() {
			log.Printf("Revert hotkey '%s' pressed. Restoring original clipboard.", hotkeyStr)
			
			// Call the revert callback
			if m.onRevert != nil {
				m.onRevert()
			}
		}
	}()

	log.Printf("Registered revert hotkey: %s", hotkeyStr)
	return nil
}

// parseHotkey converts a string hotkey combination (e.g., "ctrl+alt+v")
// into hotkey modifiers and key.
func parseHotkey(hotkeyStr string) ([]hotkey.Modifier, hotkey.Key, error) {
	parts := strings.Split(strings.ToLower(hotkeyStr), "+")
	var modifiers []hotkey.Modifier

	// Get the key (last part)
	keyStr := parts[len(parts)-1]
	key, exists := KeyMap[keyStr]
	if !exists {
		return nil, 0, fmt.Errorf("unsupported key: %s", keyStr)
	}

	// Parse modifiers (all parts except the last)
	for _, part := range parts[:len(parts)-1] {
		switch part {
		case "ctrl":
			modifiers = append(modifiers, hotkey.ModCtrl)
		case "alt":
			modifiers = append(modifiers, hotkey.ModAlt)
		case "shift":
			modifiers = append(modifiers, hotkey.ModShift)
		case "super", "win", "cmd":
			modifiers = append(modifiers, hotkey.ModWin)
		default:
			return nil, 0, fmt.Errorf("unsupported modifier: %s", part)
		}
	}

	return modifiers, key, nil
}