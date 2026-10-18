//go:build !linux

package netproxy

import (
	"context"
	"net"
)

// Start on macOS/Windows listens on loopback (there is no network namespace to splice through); the
// sandbox gets HTTP(S)_PROXY pointing at it. It returns the address for sandbox.Spec.ProxyTCP.
func Start(ctx context.Context, hosts []string, log func(string)) (sock, tcp string, stop func()) {
	if len(hosts) == 0 {
		return "", "", func() {}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", "", func() {}
	}
	pctx, cancel := context.WithCancel(ctx)
	p := &Proxy{Hosts: hosts, OnDial: func(h string, ok bool) {
		if !ok && log != nil {
			log("egress blocked: " + h)
		}
	}}
	go p.Listen(pctx, ln)
	return "", ln.Addr().String(), func() { cancel(); ln.Close() }
}
