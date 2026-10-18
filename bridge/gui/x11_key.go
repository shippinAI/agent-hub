//go:build linux

package gui

import (
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"
)

// typeText sends each rune by looking up its keysym in the server keymap, pressing shift when needed.
func (x *X11) typeText(s string) error {
	for _, r := range s {
		ks := runeKeysym(r)
		if ks == 0 {
			continue
		}
		code, shift := x.keycodeFor(ks)
		if code == 0 {
			continue
		}
		if shift {
			x.rawKey(x.shiftCode(), true)
		}
		x.rawKey(code, true)
		x.rawKey(code, false)
		if shift {
			x.rawKey(x.shiftCode(), false)
		}
	}
	return nil
}

func (x *X11) sendKey(name string) error {
	ks, mods := namedKeysym(name)
	if ks == 0 {
		return nil
	}
	var held []xproto.Keycode
	for _, m := range mods {
		c, _ := x.keycodeFor(m)
		if c != 0 {
			x.rawKey(c, true)
			held = append(held, c)
		}
	}
	code, _ := x.keycodeFor(ks)
	if code != 0 {
		x.rawKey(code, true)
		x.rawKey(code, false)
	}
	for i := len(held) - 1; i >= 0; i-- {
		x.rawKey(held[i], false)
	}
	return nil
}

func (x *X11) rawKey(code xproto.Keycode, down bool) {
	t := byte(xproto.KeyPress)
	if !down {
		t = xproto.KeyRelease
	}
	_ = xtest.FakeInputChecked(x.c, t, byte(code), 0, x.root, 0, 0, 0).Check()
}

func (x *X11) shiftCode() xproto.Keycode {
	c, _ := x.keycodeFor(0xffe1) // Shift_L
	return c
}

// keycodeFor finds the keycode producing keysym ks and whether shift is required.
func (x *X11) keycodeFor(ks uint32) (xproto.Keycode, bool) {
	setup := xproto.Setup(x.c)
	lo, hi := setup.MinKeycode, setup.MaxKeycode
	m, err := xproto.GetKeyboardMapping(x.c, lo, byte(hi-lo+1)).Reply()
	if err != nil {
		return 0, false
	}
	per := int(m.KeysymsPerKeycode)
	for i := 0; i+per <= len(m.Keysyms); i += per {
		code := xproto.Keycode(int(lo) + i/per)
		if uint32(m.Keysyms[i]) == ks {
			return code, false
		}
		if per > 1 && uint32(m.Keysyms[i+1]) == ks {
			return code, true
		}
	}
	return 0, false
}
