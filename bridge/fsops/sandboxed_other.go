//go:build !linux

package fsops

import "context"

// DoSandboxed runs in-process on macOS/Windows (the runner enforces the path against the grant; a full
// Windows build runs this under the restricted token).
func DoSandboxed(_ context.Context, _ string, _ string, _ []string, op Op) Result { return Do(op) }
