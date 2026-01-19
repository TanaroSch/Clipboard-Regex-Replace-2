//go:build !windows

package ui

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"
)

func OpenFileInDefaultApp(filePath string) error {
	log.Printf("Opening file in default app: %s (OS=%s)", filePath, runtime.GOOS)

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", filePath)
	default:
		// Assume Linux/Unix-like
		cmd = exec.Command("xdg-open", filePath)
	}

	log.Printf("Executing: %s %v", cmd.Path, cmd.Args)
	if err := cmd.Start(); err != nil {
		log.Printf("Failed to start command (%s): %v", cmd.String(), err)
		return fmt.Errorf("failed to start command (%s): %w", cmd.String(), err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
