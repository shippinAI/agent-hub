//go:build linux

package gui

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/png"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"
)

// X11 is the X11 backend. It uses GetImage for capture and the XTEST extension for synthetic input, and
// grabs ctrl+alt+Escape as a user panic hotkey that ends the session.
type X11 struct {
	c      *xgb.Conn
	root   xproto.Window
	w, h   int
	onStop func()
}

// NewX11 connects to $DISPLAY.
func NewX11(onStop func()) (*X11, error) {
	c, err := xgb.NewConn()
	if err != nil {
		return nil, err
	}
	if err := xtest.Init(c); err != nil {
		c.Close()
		return nil, fmt.Errorf("XTEST unavailable: %w", err)
	}
	setup := xproto.Setup(c)
	screen := setup.DefaultScreen(c)
	x := &X11{c: c, root: screen.Root, w: int(screen.WidthInPixels), h: int(screen.HeightInPixels), onStop: onStop}
	x.grabPanic()
	return x, nil
}

func (x *X11) Size() (int, int, error) { return x.w, x.h, nil }

func (x *X11) Close() { x.c.Close() }

func (x *X11) Screenshot(region []int, maxEdge int) (Shot, error) {
	rx, ry, rw, rh := 0, 0, x.w, x.h
	if len(region) == 4 && region[2] > 0 && region[3] > 0 {
		rx, ry, rw, rh = region[0], region[1], region[2], region[3]
	}
	img, err := xproto.GetImage(x.c, xproto.ImageFormatZPixmap, xproto.Drawable(x.root),
		int16(rx), int16(ry), uint16(rw), uint16(rh), 0xffffffff).Reply()
	if err != nil {
		return Shot{}, err
	}
	rgba := image.NewRGBA(image.Rect(0, 0, rw, rh))
	src := img.Data
	for i := 0; i+3 < len(src); i += 4 {
		px := i
		rgba.Pix[px] = src[i+2]   // R  (X sends BGRX)
		rgba.Pix[px+1] = src[i+1] // G
		rgba.Pix[px+2] = src[i]   // B
		rgba.Pix[px+3] = 255
	}
	out := scaleRGBA(rgba, maxEdge)
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return Shot{}, err
	}
	sum := sha256.Sum256(buf.Bytes())
	b := out.Bounds()
	return Shot{PNG: buf.Bytes(), SHA256: hex.EncodeToString(sum[:]), Width: b.Dx(), Height: b.Dy(), Bytes: buf.Len()}, nil
}

func (x *X11) Input(actions []Action) error {
	for _, a := range actions {
		if err := x.one(a); err != nil {
			return err
		}
	}
	return nil
}

func (x *X11) one(a Action) error {
	switch a.Type {
	case "move":
		return x.motion(a.X, a.Y)
	case "click":
		if err := x.motion(a.X, a.Y); err != nil {
			return err
		}
		return x.button(btn(a.Button), true, false)
	case "drag":
		if err := x.motion(a.X, a.Y); err != nil {
			return err
		}
		x.rawButton(btn(a.Button), true)
		x.motion(a.X2, a.Y2)
		x.rawButton(btn(a.Button), false)
		return nil
	case "scroll":
		b := uint8(5)
		if a.Amount < 0 {
			b = 4
		}
		n := a.Amount
		if n < 0 {
			n = -n
		}
		for i := 0; i < max1(n); i++ {
			x.button(b, true, false)
		}
		return nil
	case "type":
		return x.typeText(a.Text)
	case "key":
		return x.sendKey(a.Key)
	}
	return fmt.Errorf("unknown action %s", a.Type)
}
