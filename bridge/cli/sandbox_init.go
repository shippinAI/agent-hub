//go:build linux

package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"devagotchi/bridge/fsops"
	"devagotchi/bridge/netproxy"
)

// sandboxInit is pid 1 inside the bwrap sandbox (Linux). With --fwd it first starts the in-netns forwarder
// that bridges the sandbox's loopback proxy port to the host proxy socket, then execs the real command.
func sandboxInit(args []string) error {
	fs := flag.NewFlagSet("sandbox-init", flag.ContinueOnError)
	fwd := fs.String("fwd", "", "unix socket of the host egress proxy")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return fmt.Errorf("sandbox-init: no command")
	}
	if *fwd != "" {
		go func() { _ = netproxy.Forward(sandboxProxyAddr, *fwd) }()
	}
	bin, err := exec.LookPath(rest[0])
	if err != nil {
		bin = rest[0]
	}
	return syscall.Exec(bin, rest, os.Environ())
}

// sandboxProxyAddr must match sandbox.ProxyAddr; kept here to avoid importing the OS-specific const on all builds.
const sandboxProxyAddr = "127.0.0.1:3128"

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
