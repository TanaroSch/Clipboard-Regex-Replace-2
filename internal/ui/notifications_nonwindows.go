//go:build !windows

package ui

import "github.com/gen2brain/beeep"

func (n *NotificationManager) platformNotify(title, message string) error {
	// Icon path left empty on non-Windows.
	return beeep.Notify(title, message, "")
}
