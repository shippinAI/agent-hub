//go:build linux

package gui

import (
	"fmt"
	"os"
)

// Open picks the display backend from the environment. Wayland capture goes through the desktop portal;
// Wayland input is unsupported because xdg-desktop-portal exposes no synthetic-input interface here (D24).
func Open(onStop func()) (Backend, error) {
	if os.Getenv("WAYLAND_DISPLAY") != "" && os.Getenv("DISPLAY") == "" {
		return NewWayland(onStop)
	}
	if os.Getenv("DISPLAY") != "" {
		return NewX11(onStop)
	}
	return nil, fmt.Errorf("no display: set DISPLAY (X11) or WAYLAND_DISPLAY")
}
