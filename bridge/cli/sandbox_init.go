//go:build linux

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"devagotchi/bridge/fsops"
	"devagotchi/bridge/sandbox"
)

// sandboxInit is the first program inside the bwrap sandbox (Linux); see sandbox.Init.
func sandboxInit(args []string) error { return sandbox.Init(args) }

// sandboxFsop runs one fs op in-process inside the sandbox and prints the JSON result.
func sandboxFsop(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("sandbox-fsop: no op")
	}
	var op fsops.Op
	if err := json.Unmarshal([]byte(args[0]), &op); err != nil {
		return err
	}
	b, _ := json.Marshal(fsops.Do(op))
	_, _ = os.Stdout.Write(b)
	return nil
}
