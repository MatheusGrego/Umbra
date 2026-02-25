//go:build linux

package main

import "log"

// applyWindowProtection is a no-op on Linux.
// Wayland support is experimental and varies by compositor.
// X11 does not expose a reliable screenshot-prevention API.
func applyWindowProtection() error {
	log.Println("[window] screenshot protection not available on Linux")
	return nil
}
