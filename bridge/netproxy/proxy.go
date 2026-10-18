// Package netproxy is the egress allowlist: an HTTP CONNECT proxy that only dials hosts the grant names.
// On Linux it listens on a unix socket the sandbox sees; an in-sandbox forwarder (sandbox-init --fwd) bridges
// the sandbox's loopback proxy port to that socket, since the sandbox has its own empty network namespace.
// A domain allowlist cannot inspect TLS, so domain fronting can still escape it (stated in the UI, 02).
package netproxy

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"devagotchi/bridge/scope"
)

// Proxy enforces an egress allowlist for one run.
type Proxy struct {
	Hosts   []string
	OnDial  func(host string, allowed bool)
	ln      net.Listener
	dialer  net.Dialer
	closeMu sync.Once
}

// Listen starts the proxy on ln (a unix socket on Linux, a loopback TCP listener elsewhere).
func (p *Proxy) Listen(ctx context.Context, ln net.Listener) {
	p.ln = ln
	p.dialer.Timeout = 15 * time.Second
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go p.serve(ctx, c)
	}
}

func (p *Proxy) allowed(host string) bool {
	host = strings.ToLower(host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	for _, pat := range p.Hosts {
		if scope.HostOK(host, pat) {
			return true
		}
	}
	return false
}

func (p *Proxy) serve(ctx context.Context, c net.Conn) {
	defer c.Close()
	br := bufio.NewReader(c)
	req, err := http.ReadRequest(br)
	if err != nil || req.Method != http.MethodConnect {
		_, _ = io.WriteString(c, "HTTP/1.1 405 Method Not Allowed\r\n\r\n")
		return
	}
	host := req.Host
	ok := p.allowed(host)
	if p.OnDial != nil {
		p.OnDial(host, ok)
	}
	if !ok {
		_, _ = io.WriteString(c, "HTTP/1.1 403 Forbidden\r\nX-Devagotchi: host not in grant\r\n\r\n")
		return
	}
	up, err := p.dialer.DialContext(ctx, "tcp", host)
	if err != nil {
		_, _ = io.WriteString(c, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		return
	}
	defer up.Close()
	_, _ = io.WriteString(c, "HTTP/1.1 200 Connection Established\r\n\r\n")
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(up, br); done <- struct{}{} }()
	go func() { _, _ = io.Copy(c, up); done <- struct{}{} }()
	<-done
}
