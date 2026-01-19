//go:build windows

package ui

import "log"

func OpenFileInDefaultApp(filePath string) error {
	log.Println("Windows: Attempting method: ShellExecuteW API")
	err := windowsOpenFileInDefaultApp(filePath)
	if err == nil {
		log.Println("Windows Method (ShellExecuteW) succeeded.")
	} else {
		log.Printf("Windows Method (ShellExecuteW) failed: %v", err)
	}
	return err
}
