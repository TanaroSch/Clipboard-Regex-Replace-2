//go:build !windows && !linux

package hotkey

import (
	"fmt"

	"golang.design/x/hotkey"
)

// parseHotkey is not implemented on this OS.
// The project primarily targets Windows and Linux.
func parseHotkey(hotkeyStr string) ([]hotkey.Modifier, hotkey.Key, error) {
	return nil, 0, fmt.Errorf("hotkeys are not supported on this OS")
}
