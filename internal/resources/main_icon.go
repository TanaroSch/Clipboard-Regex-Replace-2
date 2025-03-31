package resources

import (
	_ "embed"
)

//go:embed icon.ico
var iconData []byte

// GetIcon returns the bytes of the embedded icon
func GetIcon() ([]byte, error) {
	if len(iconData) == 0 {
		return nil, ErrIconNotFound
	}
	return iconData, nil
}