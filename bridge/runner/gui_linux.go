//go:build linux

package runner

import (
	"context"
	"encoding/json"

	"devagotchi/bridge/gui"
	"devagotchi/bridge/wire"
)

// guiOp relays a gui.* call to the desktop helper over its owner-only socket. The helper runs in the
// logged-in session; the service (this process) may be a background daemon with no display of its own.
func (r *Runner) guiOp(_ context.Context, f wire.Frame) wire.Result {
	data, err := gui.Call(r.ConfigDir, f.Op, f.Args)
	if err != nil {
		return fail("gui_unavailable", r.Redact.String(err.Error()))
	}
	var probe struct {
		SHA string `json:"sha256"`
	}
	_ = json.Unmarshal(data, &probe)
	return wire.Result{OK: true, Data: data, OutSHA256: probe.SHA}
}
