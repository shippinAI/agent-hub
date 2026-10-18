//go:build linux

package netproxy

import (
	"context"
	"net"
	"os"
	"path/filepath"
)

// Start runs an allowlist proxy for one confined run on a 0600 unix socket the sandbox binds in. It returns
// the socket path (for sandbox.Spec.ProxySock) and a stop func. No hosts = no proxy and no network.
func Start(ctx context.Context, hosts []string, log func(string)) (sock, tcp string, stop func()) {
	if len(hosts) == 0 {
		return "", "", func() {}
	}
	dir, err := os.MkdirTemp("", "devagotchi-net-")
	if err != nil {
		return "", "", func() {}
	}
	sock = filepath.Join(dir, "proxy.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		os.RemoveAll(dir)
		return "", "", func() {}
	}
	_ = os.Chmod(sock, 0o600)
	pctx, cancel := context.WithCancel(ctx)
	p := &Proxy{Hosts: hosts, OnDial: func(h string, ok bool) {
		if !ok && log != nil {
			log("egress blocked: " + h)
		}
	}}
	go p.Listen(pctx, ln)
	return sock, "", func() { cancel(); ln.Close(); os.RemoveAll(dir) }
}
