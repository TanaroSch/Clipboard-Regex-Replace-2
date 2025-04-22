//go:build windows
// +build windows

package clipboard

import (
	"fmt"
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
	VK_LCONTROL       = 0xA2 // Use specific L/R keys if needed
    VK_SNAPSHOT       = 0x2C // PrintScreen
    KEYEVENTF_EXTENDEDKEY = 0x0001
)

// Windows INPUT structure for SendInput
type keyboardInput struct {
    Type uint32
    Ki   keyBdInput // Use nested struct for clarity and correct packing
	Padding uint64 // Add padding for 64-bit alignment
}

// keyBdInput structure nested within INPUT
type keyBdInput struct {
    WVk         uint16
    WScan       uint16
    DwFlags     uint32
    Time        uint32
    DwExtraInfo uintptr
}


// sendInputs sends a slice of INPUT structures using SendInput
func sendInputs(inputs []keyboardInput) (uintptr, error) {
	if len(inputs) == 0 {
		return 0, nil
	}
	user32 := syscall.NewLazyDLL("user32.dll")
	sendInput := user32.NewProc("SendInput")

	ret, _, err := sendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		uintptr(unsafe.Sizeof(inputs[0])),
	)

	// Check the error code from SendInput call itself
    // Note: err is often non-nil even on success ("The operation completed successfully.")
	// So, we primarily check the return value 'ret'.
	if ret != uintptr(len(inputs)) {
        errMsg := "SendInput failed"
        if err != nil && err.Error() != "The operation completed successfully." {
             errMsg = fmt.Sprintf("SendInput sent %d inputs instead of %d. Error: %v", ret, len(inputs), err)
        } else {
            errMsg = fmt.Sprintf("SendInput sent %d inputs instead of %d. GetLastError might provide details.", ret, len(inputs))
        }
		log.Println(errMsg)
		return ret, fmt.Errorf(errMsg) // Return an error object
	}

    // If ret indicates success, clear the "The operation completed successfully." error if present.
	if err != nil && err.Error() == "The operation completed successfully." {
        return ret, nil
    }

	return ret, err // Return original error if it wasn't the success message
}


// attemptPasteWithSendInput tries to paste using the SendInput Windows API
func attemptPasteWithSendInput() bool {
	log.Println("Attempting paste with SendInput API...")

	// Inputs for Ctrl+V: Press Ctrl, Press V, Release V, Release Ctrl
	inputs := []keyboardInput{
		{ // Press Left Control
			Type: INPUT_KEYBOARD,
			Ki: keyBdInput{
				WVk:     VK_LCONTROL, // Use VK_LCONTROL for specificity
				DwFlags: 0,           // Key down
			},
		},
		{ // Press V
			Type: INPUT_KEYBOARD,
			Ki: keyBdInput{
				WVk:     VK_V,
				DwFlags: 0, // Key down
			},
		},
		{ // Release V
			Type: INPUT_KEYBOARD,
			Ki: keyBdInput{
				WVk:     VK_V,
				DwFlags: KEYEVENTF_KEYUP, // Key up
			},
		},
		{ // Release Left Control
			Type: INPUT_KEYBOARD,
			Ki: keyBdInput{
				WVk:     VK_LCONTROL,
				DwFlags: KEYEVENTF_KEYUP, // Key up
			},
		},
	}

	_, err := sendInputs(inputs)
	if err != nil {
		log.Printf("SendInput method failed: %v", err)
		return false
	}

	log.Println("SendInput method appears successful.")
	return true
}


// attemptPasteWithKeyBdEvent tries to paste using the keybd_event Windows API (legacy method)
// Generally less reliable than SendInput, especially in modern Windows versions or specific apps.
func attemptPasteWithKeyBdEvent() bool {
    log.Println("Attempting paste with keybd_event API (Legacy Fallback)...")
	user32 := syscall.NewLazyDLL("user32.dll")
	keybd_event := user32.NewProc("keybd_event")

	// Simulate Ctrl down, V down, V up, Ctrl up
	// keybd_event(bVk, bScan, dwFlags, dwExtraInfo)
	// Using 0 for bScan, letting Windows map VK code
	_, _, err1 := keybd_event.Call(VK_CONTROL, 0, 0, 0)           // Press Ctrl
	time.Sleep(10 * time.Millisecond)                             // Small delay between keys
	_, _, err2 := keybd_event.Call(VK_V, 0, 0, 0)                 // Press V
	time.Sleep(10 * time.Millisecond)
	_, _, err3 := keybd_event.Call(VK_V, 0, KEYEVENTF_KEYUP, 0)   // Release V
	time.Sleep(10 * time.Millisecond)
	_, _, err4 := keybd_event.Call(VK_CONTROL, 0, KEYEVENTF_KEYUP, 0) // Release Ctrl

    // Check for errors, being mindful of the "success" error message
    errCount := 0
    for _, err := range []error{err1, err2, err3, err4} {
        if err != nil && err.Error() != "The operation completed successfully." {
             log.Printf("keybd_event call failed: %v", err)
             errCount++
        }
    }

	if errCount > 0 {
        log.Println("keybd_event method failed.")
        return false
    }

    log.Println("keybd_event method completed (no explicit errors).")
	return true
}


// attemptPasteWithPowershell tries to paste using PowerShell SendKeys
// This is often blocked by security policies or might focus the PowerShell window.
func attemptPasteWithPowershell() bool {
    log.Println("Attempting paste with PowerShell SendKeys (Fallback)...")
	// Create a PowerShell script that simulates Ctrl+V keypress using SendWait
	// SendWait is generally better than Send for reliability but can hang.
	psScript := `
Add-Type -AssemblyName System.Windows.Forms
[System.Windows.Forms.SendKeys]::SendWait("^v")
`
    // Use -NoProfile and -NonInteractive for cleaner execution
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
    // Capture output to see potential errors from PowerShell itself
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("PowerShell paste command failed: %v\nOutput:\n%s", err, string(output))
		return false
	}

    log.Println("PowerShell paste command executed successfully.")
	return true
}

// simulatePlatformPaste tries multiple methods to simulate Ctrl+V in Windows.
// This function provides the implementation for Windows builds.
func simulatePlatformPaste() {
	log.Println("Attempting to simulate paste on Windows using multiple methods...")

	// Add a small delay before trying to paste to allow focus changes etc.
	time.Sleep(150 * time.Millisecond) // Slightly increased delay

	// --- Method 1: SendInput API (Most reliable generally) ---
	if attemptPasteWithSendInput() {
		log.Println("Paste simulation via SendInput SUCCEEDED.")
		return // Success! Stop trying other methods.
	}
	log.Println("Paste simulation via SendInput failed. Trying next method...")
	time.Sleep(50 * time.Millisecond) // Delay before next attempt


	// --- Method 2: keybd_event API (Legacy fallback) ---
	if attemptPasteWithKeyBdEvent() {
		log.Println("Paste simulation via keybd_event SUCCEEDED.")
		return // Success! Stop trying other methods.
	}
	log.Println("Paste simulation via keybd_event failed. Trying next method...")
	time.Sleep(50 * time.Millisecond)


	// --- Method 3: PowerShell SendKeys (Less reliable, often blocked) ---
	// if attemptPasteWithPowershell() {
	// 	log.Println("Paste simulation via PowerShell SUCCEEDED.")
	// 	return // Success!
	// }
    // log.Println("Paste simulation via PowerShell failed.")
    // Commented out by default as it's often problematic. Uncomment to enable.


	// --- Failure ---
	log.Println("All Windows paste simulation methods failed!")
	// Optionally, display a notification to the user about the failure?
	// ui.ShowNotification("Paste Failed", "Could not simulate Ctrl+V paste action.")
}