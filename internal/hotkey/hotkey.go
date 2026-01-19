package hotkey

import (
	"fmt"
	"log"
	"sync"

	"github.com/TanaroSch/clipboard-regex-replace/internal/config"
	"golang.design/x/hotkey"
)

// Manager handles registration and lifecycle of global hotkeys
type Manager struct {
	mu                sync.RWMutex // Protects registeredHotkeys and quitChannels
	config            *config.Config
	registeredHotkeys map[string][]*hotkey.Hotkey
	quitChannels      map[string]chan struct{} // Channels to signal goroutines to stop
	onTrigger         func(string, bool)       // hotkeyStr, isReverse
	onRevert          func()
}

// NewManager creates a new hotkey manager
func NewManager(cfg *config.Config, onTrigger func(string, bool), onRevert func()) *Manager {
	return &Manager{
		config:            cfg,
		registeredHotkeys: make(map[string][]*hotkey.Hotkey),
		quitChannels:      make(map[string]chan struct{}),
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
	m.mu.Lock()
	defer m.mu.Unlock()

	// Signal all goroutines to stop
	for key, quitCh := range m.quitChannels {
		close(quitCh)
		log.Printf("Signaled goroutine to stop for hotkey: %s", key)
	}

	// Unregister all hotkeys
	for _, hks := range m.registeredHotkeys {
		for _, hk := range hks {
			_ = hk.Unregister()
		}
	}

	// Clear maps
	m.registeredHotkeys = make(map[string][]*hotkey.Hotkey)
	m.quitChannels = make(map[string]chan struct{})
}

// registerProfileHotkey registers a hotkey for a profile
func (m *Manager) registerProfileHotkey(profile config.ProfileConfig, hotkeyStr string, isReverse bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Skip if already registered
	if _, exists := m.registeredHotkeys[hotkeyStr]; exists {
		return nil
	}

	// Parse and register the hotkey
	modifiers, key, err := parseHotkey(hotkeyStr)
	if err != nil {
		return err
	}

	modifierSets := expandModifiers(modifiers)
	var hks []*hotkey.Hotkey
	for _, mods := range modifierSets {
		hk := hotkey.New(mods, key)
		if err := hk.Register(); err != nil {
			// Best-effort rollback for any already registered variants.
			for _, registered := range hks {
				_ = registered.Unregister()
			}
			return err
		}
		hks = append(hks, hk)
	}

	// Create quit channel for this hotkey's goroutine
	quitCh := make(chan struct{})

	// Store in our tracking maps
	m.registeredHotkeys[hotkeyStr] = hks
	m.quitChannels[hotkeyStr] = quitCh

	// Direction suffix for logging
	directionSuffix := ""
	if isReverse {
		directionSuffix = " (reverse)"
	}

	// Create listeners for all registered variants of this hotkey.
	for idx, hk := range hks {
		go func(hotkeyStr string, isReverse bool, hk *hotkey.Hotkey, quitCh chan struct{}, profileName string, directionSuffix string, variantIndex int) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("RECOVERED FROM PANIC IN HOTKEY LISTENER (%s, variant %d): %v", hotkeyStr, variantIndex, r)
				}
			}()

			for {
				select {
				case <-quitCh:
					log.Printf("Hotkey listener for '%s' (variant %d) stopping", hotkeyStr, variantIndex)
					return
				case <-hk.Keydown():
					log.Printf("Hotkey '%s' pressed (variant %d). Processing clipboard using profile: %s%s",
						hotkeyStr, variantIndex, profileName, directionSuffix)

					// Call the callback function
					if m.onTrigger != nil {
						m.onTrigger(hotkeyStr, isReverse)
					}
				}
			}
		}(hotkeyStr, isReverse, hk, quitCh, profile.Name, directionSuffix, idx)
	}

	log.Printf("Registered hotkey '%s' for profile: %s%s",
		hotkeyStr, profile.Name, directionSuffix)

	return nil
}

// registerRevertHotkey registers a global hotkey for reverting the clipboard
func (m *Manager) registerRevertHotkey(hotkeyStr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Skip if already registered
	if _, exists := m.registeredHotkeys[hotkeyStr]; exists {
		return nil
	}

	// Parse the hotkey
	modifiers, key, err := parseHotkey(hotkeyStr)
	if err != nil {
		return err
	}

	modifierSets := expandModifiers(modifiers)
	var hks []*hotkey.Hotkey
	for _, mods := range modifierSets {
		hk := hotkey.New(mods, key)
		if err := hk.Register(); err != nil {
			for _, registered := range hks {
				_ = registered.Unregister()
			}
			return err
		}
		hks = append(hks, hk)
	}

	// Create quit channel for this hotkey's goroutine
	quitCh := make(chan struct{})

	// Store in our tracking maps
	m.registeredHotkeys[hotkeyStr] = hks
	m.quitChannels[hotkeyStr] = quitCh

	// Create listeners for all registered variants of this hotkey.
	for idx, hk := range hks {
		go func(hotkeyStr string, hk *hotkey.Hotkey, quitCh chan struct{}, variantIndex int) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("RECOVERED FROM PANIC IN REVERT HOTKEY LISTENER (%s, variant %d): %v", hotkeyStr, variantIndex, r)
				}
			}()

			for {
				select {
				case <-quitCh:
					log.Printf("Revert hotkey listener for '%s' (variant %d) stopping", hotkeyStr, variantIndex)
					return
				case <-hk.Keydown():
					log.Printf("Revert hotkey '%s' pressed (variant %d). Restoring original clipboard.", hotkeyStr, variantIndex)

					// Call the revert callback
					if m.onRevert != nil {
						m.onRevert()
					}
				}
			}
		}(hotkeyStr, hk, quitCh, idx)
	}

	log.Printf("Registered revert hotkey: %s", hotkeyStr)
	return nil
}
