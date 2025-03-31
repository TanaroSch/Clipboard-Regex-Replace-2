//go:build windows
// +build windows

package clipboard

import (
	"log"
	"os/exec"
	"syscall"
	"time"
	"unsafe"
)

// Windows constants for keyboard events
const (
	INPUT_KEYBOARD    = 1
	KEYEVENTF_KEYUP   = 0x0002
	VK_CONTROL        = 0x11
	VK_V              = 0x56
)

// Windows INPUT structure for SendInput
type keyboardInput struct {
	Type uint32
	Ki   struct {
		WVk         uint16
		WScan       uint16
		DwFlags     uint32
		Time        uint32
		DwExtraInfo uintptr
		Padding1    uint32
		Padding2    uint32
		Padding3    uint32
	}
}

// attemptPasteWithSendInput tries to paste using the SendInput Windows API
func attemptPasteWithSendInput() bool {
	user32 := syscall.NewLazyDLL("user32.dll")
	sendInput := user32.NewProc("SendInput")

	// Create slice of 4 INPUT structures (Ctrl down, V down, V up, Ctrl up)
	inputs := make([]keyboardInput, 4)

	// Key down events
	inputs[0].Type = INPUT_KEYBOARD
	inputs[0].Ki.WVk = VK_CONTROL
	inputs[0].Ki.WScan = 0
	inputs[0].Ki.DwFlags = 0
	inputs[0].Ki.Time = 0
	inputs[0].Ki.DwExtraInfo = 0

	inputs[1].Type = INPUT_KEYBOARD
	inputs[1].Ki.WVk = VK_V
	inputs[1].Ki.WScan = 0
	inputs[1].Ki.DwFlags = 0
	inputs[1].Ki.Time = 0
	inputs[1].Ki.DwExtraInfo = 0

	// Key up events
	inputs[2].Type = INPUT_KEYBOARD
	inputs[2].Ki.WVk = VK_V
	inputs[2].Ki.WScan = 0
	inputs[2].Ki.DwFlags = KEYEVENTF_KEYUP
	inputs[2].Ki.Time = 0
	inputs[2].Ki.DwExtraInfo = 0

	inputs[3].Type = INPUT_KEYBOARD
	inputs[3].Ki.WVk = VK_CONTROL
	inputs[3].Ki.WScan = 0
	inputs[3].Ki.DwFlags = KEYEVENTF_KEYUP
	inputs[3].Ki.Time = 0
	inputs[3].Ki.DwExtraInfo = 0

	// Send all inputs at once
	ret, _, err := sendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		uintptr(unsafe.Sizeof(inputs[0])),
	)

	if ret != uintptr(len(inputs)) {
		log.Printf("SendInput failed, sent %d inputs instead of %d: %v", ret, len(inputs), err)
		return false
	}
	
	return true
}

// attemptPasteWithKeyBdEvent tries to paste using the keybd_event Windows API (original method)
func attemptPasteWithKeyBdEvent() bool {
	user32 := syscall.NewLazyDLL("user32.dll")
	keybd_event := user32.NewProc("keybd_event")
	
	// VK_CONTROL = 0x11, VK_V = 0x56
	keybd_event.Call(VK_CONTROL, 0, 0, 0)           // Press Ctrl
	keybd_event.Call(VK_V, 0, 0, 0)                 // Press V
	keybd_event.Call(VK_V, 0, KEYEVENTF_KEYUP, 0)   // Release V
	keybd_event.Call(VK_CONTROL, 0, KEYEVENTF_KEYUP, 0) // Release Ctrl
	
	return true
}

// attemptPasteWithPowershell tries to paste using PowerShell
func attemptPasteWithPowershell() bool {
	// Create a PowerShell script that simulates Ctrl+V keypress
	psScript := `
	Add-Type -AssemblyName System.Windows.Forms
	[System.Windows.Forms.SendKeys]::SendWait("^v")
	`
	
	cmd := exec.Command("powershell", "-Command", psScript)
	if err := cmd.Run(); err != nil {
		log.Printf("PowerShell paste failed: %v", err)
		return false
	}
	
	return true
}

// WindowsPaste tries multiple methods to simulate Ctrl+V in Windows
func WindowsPaste() {
	log.Println("Attempting to paste using multiple methods...")
	
	// Add a small delay before trying to paste
	time.Sleep(100 * time.Millisecond)
	
	// Try method 1: SendInput API (most reliable)
	log.Println("Method 1: Using SendInput API...")
	if attemptPasteWithSendInput() {
		log.Println("SendInput method completed.")
		return
	}
	
	// If that fails, try method 2: keybd_event API (original method)
	log.Println("Method 2: Using keybd_event API...")
	if attemptPasteWithKeyBdEvent() {
		log.Println("keybd_event method completed.")
		return
	}
	
	// If that fails too, try method 3: PowerShell
	log.Println("Method 3: Using PowerShell...")
	if attemptPasteWithPowershell() {
		log.Println("PowerShell method completed.")
		return
	}
	
	log.Println("All paste methods failed!")
}