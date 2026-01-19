//go:build !linux

package hotkey

import "golang.design/x/hotkey"

func expandModifiers(modifiers []hotkey.Modifier) [][]hotkey.Modifier {
	return [][]hotkey.Modifier{modifiers}
}
