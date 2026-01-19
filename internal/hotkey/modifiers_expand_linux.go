//go:build linux

package hotkey

import "golang.design/x/hotkey"

// X11 lock masks that commonly interfere with XGrabKey.
// CapsLock is LockMask (1<<1) and NumLock is often Mod2.
const (
	linuxCapsLockMask hotkey.Modifier = 1 << 1
)

func expandModifiers(modifiers []hotkey.Modifier) [][]hotkey.Modifier {
	// Register the same hotkey for common lock-modifier states so it still
	// triggers when NumLock/CapsLock are enabled.
	base := append([]hotkey.Modifier(nil), modifiers...)
	withNum := append(append([]hotkey.Modifier(nil), modifiers...), hotkey.Mod2)
	withCaps := append(append([]hotkey.Modifier(nil), modifiers...), linuxCapsLockMask)
	withBoth := append(append([]hotkey.Modifier(nil), modifiers...), hotkey.Mod2, linuxCapsLockMask)

	return [][]hotkey.Modifier{base, withNum, withCaps, withBoth}
}
