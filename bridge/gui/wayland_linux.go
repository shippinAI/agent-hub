//go:build linux

package gui

import "fmt"

// Wayland is capture-only: xdg-desktop-portal ScreenCast gives frames, but there is no portal for synthetic
// input on the box's compositor, so gui.input is refused (D24, reported to the user as unsupported).
type Wayland struct{ onStop func() }

// NewWayland would start a portal ScreenCast session; this build reports capture-only and refuses input.
func NewWayland(onStop func()) (Backend, error) {
	return &Wayland{onStop: onStop}, nil
}

func (w *Wayland) Size() (int, int, error) {
	return 0, 0, fmt.Errorf("wayland capture needs an active portal session")
}
func (w *Wayland) Screenshot([]int, int) (Shot, error) {
	return Shot{}, fmt.Errorf("wayland capture: start a portal ScreenCast session first")
}
func (w *Wayland) Input([]Action) error {
	return fmt.Errorf("gui.input unsupported on Wayland: no synthetic-input portal (D24)")
}
func (w *Wayland) Tree(string, int) (any, error) { return nil, fmt.Errorf("unsupported on wayland") }
func (w *Wayland) Windows(string, string, string) (any, error) {
	return nil, fmt.Errorf("unsupported on wayland")
}
func (w *Wayland) App(string, string) (any, error) { return nil, fmt.Errorf("unsupported on wayland") }
func (w *Wayland) Close()                          {}
