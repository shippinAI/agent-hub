//go:build linux

package gui

import (
	"os/exec"
	"strings"

	"github.com/jezek/xgb/xproto"
)

// grabPanic grabs ctrl+alt+Escape so the person at the keyboard can always kill the session.
func (x *X11) grabPanic() {
	code, _ := x.keycodeFor(0xff1b) // Escape
	if code == 0 {
		return
	}
	const ctrlAlt = xproto.ModMaskControl | xproto.ModMask1
	_ = xproto.GrabKeyChecked(x.c, true, x.root, ctrlAlt, code,
		xproto.GrabModeAsync, xproto.GrabModeAsync).Check()
	go x.watchPanic()
}

func (x *X11) watchPanic() {
	for {
		ev, err := x.c.WaitForEvent()
		if err != nil || ev == nil {
			return
		}
		if _, ok := ev.(xproto.KeyPressEvent); ok && x.onStop != nil {
			x.onStop()
			return
		}
	}
}

// Tree, Windows and App lean on wmctrl/xdotool when present (02 "Unverified" X11 path). The grant's app
// filter is enforced by the hub before the op reaches here; these only act within the running session.
func (x *X11) Tree(window string, depth int) (any, error) {
	return map[string]any{"note": "accessibility tree requires AT-SPI; not available in this build", "window": window}, nil
}

func (x *X11) Windows(op, window, app string) (any, error) {
	switch op {
	case "", "list":
		out, _ := exec.Command("wmctrl", "-l").Output()
		var ws []map[string]string
		for _, ln := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			f := strings.Fields(ln)
			if len(f) >= 4 {
				ws = append(ws, map[string]string{"id": f[0], "title": strings.Join(f[3:], " ")})
			}
		}
		return map[string]any{"windows": ws}, nil
	case "focus":
		return nil, exec.Command("wmctrl", "-i", "-a", window).Run()
	case "minimise", "minimize":
		return nil, exec.Command("xdotool", "windowminimize", window).Run()
	}
	return map[string]any{"op": op}, nil
}

func (x *X11) App(op, app string) (any, error) {
	switch op {
	case "launch":
		c := exec.Command(app)
		return map[string]any{"launched": app}, c.Start()
	case "quit":
		return nil, exec.Command("pkill", "-f", app).Run()
	}
	return map[string]any{"op": op}, nil
}
