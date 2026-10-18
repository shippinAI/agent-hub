//go:build !linux

package gui

import "fmt"

// Open on macOS/Windows returns the native backend; those are documented stubs in this build (ScreenCaptureKit
// / Windows.Graphics.Capture are not wired, a stated deviation). The cross-compiled binaries carry the code
// path so a hardware build can fill them in.
func Open(onStop func()) (Backend, error) {
	return nil, fmt.Errorf("gui helper is not implemented for this OS in this build")
}
