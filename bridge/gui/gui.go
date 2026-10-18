// Package gui is the desktop helper: it runs in the user's logged-in session and services gui.* ops
// (screenshot, input, tree, window, app) for exactly one live session. Linux has an X11 backend (XTEST +
// XGrabKey panic hotkey) and a Wayland portal backend (capture only; input unsupported, D24). The helper
// never runs on a secure/login desktop and never requests UI-access elevation (invariant 01).
package gui

import (
	"context"
	"encoding/json"
	"fmt"
)

// Action is one gui.input step in screenshot coordinates.
type Action struct {
	Type   string `json:"type"` // move, click, drag, scroll, type, key
	X      int    `json:"x"`
	Y      int    `json:"y"`
	X2     int    `json:"x2,omitempty"`
	Y2     int    `json:"y2,omitempty"`
	Button int    `json:"button,omitempty"`
	Amount int    `json:"amount,omitempty"`
	Text   string `json:"text,omitempty"`
	Key    string `json:"key,omitempty"`
}

// Shot is a screenshot result; PNG is sent to the agent, the rest is audited.
type Shot struct {
	PNG    []byte `json:"png"`
	SHA256 string `json:"sha256"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Bytes  int    `json:"bytes"`
}

// Backend is a display server implementation.
type Backend interface {
	Size() (int, int, error)
	Screenshot(region []int, maxEdge int) (Shot, error)
	Input(actions []Action) error
	Tree(window string, depth int) (any, error)
	Windows(op, window, app string) (any, error)
	App(op, app string) (any, error)
	Close()
}

// Op is a decoded gui.* request from the bridge.
type Op struct {
	Op      string           `json:"-"`
	Region  []int            `json:"region,omitempty"`
	MaxEdge int              `json:"max_edge,omitempty"`
	Actions []map[string]any `json:"actions,omitempty"`
	Window  string           `json:"window,omitempty"`
	WinOp   string           `json:"op,omitempty"`
	App     string           `json:"app,omitempty"`
	Depth   int              `json:"depth,omitempty"`
}

// Handle runs one op against the backend and returns a JSON payload.
func Handle(_ context.Context, b Backend, op string, raw json.RawMessage) (json.RawMessage, error) {
	var in Op
	_ = json.Unmarshal(raw, &in)
	switch op {
	case "gui.session", "gui.observe":
		w, h, err := b.Size()
		if err != nil {
			return nil, err
		}
		return mj(map[string]any{"width": w, "height": h, "ready": true}), nil
	case "gui.screenshot":
		s, err := b.Screenshot(in.Region, in.MaxEdge)
		if err != nil {
			return nil, err
		}
		return mj(s), nil
	case "gui.input":
		acts, err := decodeActions(in.Actions)
		if err != nil {
			return nil, err
		}
		if err := b.Input(acts); err != nil {
			return nil, err
		}
		return mj(map[string]any{"applied": len(acts)}), nil
	case "gui.tree":
		t, err := b.Tree(in.Window, in.Depth)
		if err != nil {
			return nil, err
		}
		return mj(t), nil
	case "gui.window":
		r, err := b.Windows(in.WinOp, in.Window, in.App)
		if err != nil {
			return nil, err
		}
		return mj(r), nil
	case "gui.app":
		r, err := b.App(in.WinOp, in.App)
		if err != nil {
			return nil, err
		}
		return mj(r), nil
	}
	return nil, fmt.Errorf("unknown gui op %s", op)
}

func decodeActions(in []map[string]any) ([]Action, error) {
	b, _ := json.Marshal(in)
	var out []Action
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func mj(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
