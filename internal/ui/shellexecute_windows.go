// internal/ui/shellexecute_windows.go
//go:build windows
// +build windows

package ui

import (
	"fmt"
	"syscall"
	"unsafe"
)

// --- Add Windows API constants ---
const (
	SW_SHOWNORMAL = 1
)

// --- ShellExecute func definition (Load DLL and Proc) ---
var (
	// Load shell32 lazily
	shell32           = syscall.NewLazyDLL("shell32.dll")
	procShellExecuteW = shell32.NewProc("ShellExecuteW")
)

// ShellExecute simplifies calling the Windows ShellExecuteW API.
// It attempts to perform an operation (like 'open') on a specified file.
// This function is intended for Windows only.
func ShellExecute(hwnd uintptr, verb, file, params, dir string, showCmd int32) (err error) {
	// Convert Go strings to UTF-16 pointers required by Windows API
	lpVerb, err := syscall.UTF16PtrFromString(verb)
	if err != nil {
		return fmt.Errorf("failed to convert verb to UTF16Ptr: %w", err)
	}
	lpFile, err := syscall.UTF16PtrFromString(file)
	if err != nil {
		return fmt.Errorf("failed to convert file path to UTF16Ptr: %w", err)
	}
	// Handle potentially empty strings for params and dir safely
	var lpParams *uint16
	if params != "" {
		lpParams, err = syscall.UTF16PtrFromString(params)
		if err != nil {
			return fmt.Errorf("failed to convert params to UTF16Ptr: %w", err)
		}
	}
	var lpDir *uint16
	if dir != "" {
		lpDir, err = syscall.UTF16PtrFromString(dir)
		if err != nil {
			return fmt.Errorf("failed to convert dir to UTF16Ptr: %w", err)
		}
	}

	// Call the ShellExecuteW procedure
	ret, _, callErr := procShellExecuteW.Call(
		hwnd,
		uintptr(unsafe.Pointer(lpVerb)),
		uintptr(unsafe.Pointer(lpFile)),
		uintptr(unsafe.Pointer(lpParams)), // Use potentially nil pointer
		uintptr(unsafe.Pointer(lpDir)),    // Use potentially nil pointer
		uintptr(showCmd),
	)

	// Values > 32 indicate success (per ShellExecute documentation)
	// Instance handle is returned on success, error code otherwise.
	if ret <= 32 {
		// Combine the return code with any error from the Call itself
		errMsg := ""
		if callErr != nil {
			errMsg = callErr.Error()
		}
		// Check specifically for the "operation completed successfully" error which isn't a real error.
		if callErr != nil && errMsg != "The operation completed successfully." {
			err = fmt.Errorf("ShellExecuteW failed with return code %d and call error: %w", ret, callErr)
		} else {
			// Map common error codes if possible, otherwise just show the code
			err = fmt.Errorf("ShellExecuteW failed with return code %d (see ShellExecute docs for meaning)", ret)
		}
	} else {
		// Success, clear any potential "success" error message from Call
		err = nil
	}

	return err
}

// windowsOpenFileInDefaultApp uses ShellExecuteW to open a file.
func windowsOpenFileInDefaultApp(filePath string) error {
	// Use "open" verb to open the file with its default application.
	// Pass 0 for hwnd (no parent window), empty strings for params and dir.
	// Use SW_SHOWNORMAL to show the application window normally.
    return ShellExecute(0, "open", filePath, "", "", SW_SHOWNORMAL)
}