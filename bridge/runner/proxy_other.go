//go:build !linux

package runner

import (
	"context"
	"net"

	"devagotchi/bridge/netproxy"
	"devagotchi/bridge/sandbox"
	"devagotchi/bridge/scope"
)

// startProxy on macOS/Windows listens on loopback (no network namespace to splice through) and the sandbox
// is told to use it via HTTP(S)_PROXY.
func (r *Runner) startProxy(ctx context.Context, spec *sandbox.Spec, sel scope.Sel) func() {
	hosts, ok := sel.Strs("hosts")
	if !ok || len(hosts) == 0 {
		return nil
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil
	}
	pctx, cancel := context.WithCancel(ctx)
	p := &netproxy.Proxy{Hosts: hosts, OnDial: func(h string, ok bool) {
		if !ok {
			r.Log("egress blocked: " + h)
		}
	}}
	go p.Listen(pctx, ln)
	spec.ProxyTCP = ln.Addr().String()
	return func() { cancel(); ln.Close() }
}
