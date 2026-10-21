//go:build linux

package runner

import (
	"context"
	"net"
	"os"
	"path/filepath"

	"devagotchi/bridge/netproxy"
	"devagotchi/bridge/sandbox"
	"devagotchi/bridge/scope"
)

// startProxy starts the egress allowlist on a unix socket the sandbox will see and points the spec at it.
// No hosts in the selector means no network at all (the socket is never created).
func (r *Runner) startProxy(ctx context.Context, spec *sandbox.Spec, sel scope.Sel) func() {
	hosts, ok := sel.Strs("hosts")
	if !ok || len(hosts) == 0 {
		return nil
	}
	dir, err := os.MkdirTemp("", "devagotchi-net-")
	if err != nil {
		return nil
	}
	sock := filepath.Join(dir, "proxy.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		os.RemoveAll(dir)
		return nil
	}
	_ = os.Chmod(sock, 0o600)
	pctx, cancel := context.WithCancel(ctx)
	p := &netproxy.Proxy{Hosts: hosts, OnDial: func(h string, ok bool) {
		if !ok {
			r.Log("egress blocked: " + h)
		}
	}}
	go p.Listen(pctx, ln)
	spec.ProxySock = sock
	return func() { cancel(); ln.Close(); os.RemoveAll(dir) }
}
