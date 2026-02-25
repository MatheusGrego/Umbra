//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	// WDA_EXCLUDEFROMCAPTURE prevents the window from appearing in screenshots,
	// screen recordings (OBS, ShareX, native snipping tools), and window capture.
	WDA_EXCLUDEFROMCAPTURE = 0x00000011
)

var (
	user32                     = syscall.NewLazyDLL("user32.dll")
	procSetWindowDisplayAffinity = user32.NewProc("SetWindowDisplayAffinity")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
)

// applyWindowProtection sets WDA_EXCLUDEFROMCAPTURE on the current foreground window.
// Must be called after the Wails window is fully created and visible.
func applyWindowProtection() error {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return fmt.Errorf("window protection: could not get foreground window handle")
	}
	ret, _, err := procSetWindowDisplayAffinity.Call(hwnd, WDA_EXCLUDEFROMCAPTURE)
	if ret == 0 {
		return fmt.Errorf("window protection: SetWindowDisplayAffinity failed: %w", err)
	}
	return nil
}

// windowHandle returns the HWND as uintptr for use with other Win32 calls.
func windowHandle() uintptr {
	hwnd, _, _ := procGetForegroundWindow.Call()
	return hwnd
}

// zeroMemory securely wipes a byte slice — useful before freeing sensitive buffers.
func zeroMemory(b []byte) {
	for i := range b {
		b[i] = 0
	}
	// Prevent compiler optimisation from eliding the loop.
	_ = unsafe.Pointer(&b)
}
