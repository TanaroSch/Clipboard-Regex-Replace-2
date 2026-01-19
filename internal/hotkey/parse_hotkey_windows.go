//go:build windows

package hotkey

import (
	"fmt"
	"strings"

	"golang.design/x/hotkey"
)

// parseHotkey converts a string hotkey combination (e.g., "ctrl+alt+v")
// into golang.design/x/hotkey modifiers and key.
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
		case "super", "win":
			modifiers = append(modifiers, hotkey.ModWin)
		case "cmd":
			// On Windows, treat cmd as the Windows key.
			modifiers = append(modifiers, hotkey.ModWin)
		default:
			return nil, 0, fmt.Errorf("unsupported modifier: %s", part)
		}
	}

	return modifiers, key, nil
}
