//go:build !linux

package runner

import (
	"context"

	"devagotchi/bridge/wire"
)

func (r *Runner) guiOp(_ context.Context, f wire.Frame) wire.Result {
	return fail("gui_unavailable", "gui helper is not implemented on this OS in this build")
}
