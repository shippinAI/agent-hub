//go:build linux

package sandbox

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"devagotchi/bridge/netproxy"
)

// Init is `sandbox-init`, the first program inside bwrap. With --fwd it opens the in-namespace proxy port,
// serves it in this process and runs the command as a child, so the forwarder lives as long as the command.
// (Exec'ing the command would replace this process and take the forwarder with it.) Without --fwd it execs.
// It returns only on error; on success it exits with the command's status.
func Init(args []string) error {
	fs := flag.NewFlagSet("sandbox-init", flag.ContinueOnError)
	fwd := fs.String("fwd", "", "unix socket of the host egress proxy")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return fmt.Errorf("sandbox-init: no command")
	}
	bin, err := exec.LookPath(rest[0])
	if err != nil {
		bin = rest[0]
	}
	if *fwd == "" {
		return syscall.Exec(bin, rest, os.Environ())
	}
	ln, err := net.Listen("tcp", ProxyAddr)
	if err != nil {
		return fmt.Errorf("sandbox-init: egress forwarder: %w", err)
	}
	go func() { _ = netproxy.ForwardOn(ln, *fwd) }()
	c := exec.Command(bin, rest[1:]...)
	c.Args[0] = rest[0]
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Start(); err != nil {
		return err
	}
	sig := make(chan os.Signal, 4)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT)
	go func() {
		for s := range sig {
			_ = c.Process.Signal(s)
		}
	}()
	err = c.Wait()
	var ee *exec.ExitError
	switch {
	case err == nil:
		os.Exit(0)
	case errors.As(err, &ee):
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			os.Exit(128 + int(ws.Signal()))
		}
		os.Exit(ee.ExitCode())
	}
	return err
}
