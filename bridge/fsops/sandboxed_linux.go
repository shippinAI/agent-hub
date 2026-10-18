//go:build linux

package fsops

import (
	"context"
	"encoding/json"
	"os/exec"

	"devagotchi/bridge/sandbox"
)

// DoSandboxed runs the op re-executed inside the same bwrap confinement (self "sandbox-init fsop"), so the
// grant's path binds apply to file access too. If the sandbox cannot be built it falls back to in-process
// Do (the runner has already checked the path against the grant).
func DoSandboxed(ctx context.Context, self, level string, paths []string, op Op) Result {
	if self == "" || sandbox.Kind() == "none" {
		return Do(op)
	}
	area := paths
	if op.Path != "" {
		area = append(area, op.Path)
	}
	if op.To != "" {
		area = append(area, op.To)
	}
	in, _ := json.Marshal(op)
	spec := sandbox.Spec{Level: level, Paths: area, Argv: []string{"/run/devagotchi-init", "sandbox-fsop", string(in)}, Self: self}
	cmd, err := sandbox.Command(spec)
	if err != nil {
		return Do(op)
	}
	out, err := cmd.Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return Do(op)
		}
	}
	var r Result
	if json.Unmarshal(out, &r) != nil {
		return Do(op)
	}
	return r
}
