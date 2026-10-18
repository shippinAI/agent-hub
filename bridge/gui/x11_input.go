//go:build linux

package gui

import (
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"
)

func (x *X11) motion(px, py int) error {
	return xtest.FakeInputChecked(x.c, xproto.MotionNotify, 0, 0, x.root, int16(px), int16(py), 0).Check()
}

func (x *X11) rawButton(b uint8, down bool) {
	t := byte(xproto.ButtonPress)
	if !down {
		t = xproto.ButtonRelease
	}
	_ = xtest.FakeInputChecked(x.c, t, b, 0, x.root, 0, 0, 0).Check()
}

func (x *X11) button(b uint8, down, _ bool) error {
	x.rawButton(b, true)
	x.rawButton(b, false)
	return nil
}

func btn(b int) uint8 {
	if b <= 0 {
		return 1
	}
	return uint8(b)
}

func max1(n int) int {
	if n <= 0 {
		return 1
	}
	return n
}
