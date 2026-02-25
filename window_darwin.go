//go:build darwin

package main

// applyWindowProtection sets NSWindowSharingNone via Objective-C bridging.
// This prevents the window from appearing in screenshots or screen recordings.
//
// The CGO call is:
//   [[NSApplication sharedApplication] windows]  → find our window
//   [window setSharingType: NSWindowSharingNone]
//
// In a Wails app, the simplest approach is to call this from the JS side
// via wails.EventsEmit, or to use runtime.WindowSetAlwaysOnTop + the approach below.

// #cgo CFLAGS: -x objective-c
// #cgo LDFLAGS: -framework Cocoa
// #import <Cocoa/Cocoa.h>
//
// void applyNSWindowProtection() {
//     NSArray *windows = [[NSApplication sharedApplication] windows];
//     for (NSWindow *window in windows) {
//         [window setSharingType: NSWindowSharingNone];
//     }
// }
import "C"

func applyWindowProtection() error {
	C.applyNSWindowProtection()
	return nil
}
