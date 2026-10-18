//go:build linux

package netproxy

import (
	"io"
	"net"
	"sync"
)

// Forward runs inside the sandbox's network namespace (sandbox-init --fwd): it listens on the loopback
// proxy port and splices every connection to the unix socket the real proxy serves on the host side.
func Forward(listen, sock string) error {
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	for {
		c, err := ln.Accept()
		if err != nil {
			return err
		}
		go func() {
			defer c.Close()
			up, err := net.Dial("unix", sock)
			if err != nil {
				return
			}
			defer up.Close()
			var wg sync.WaitGroup
			wg.Add(2)
			go func() { defer wg.Done(); _, _ = io.Copy(up, c) }()
			go func() { defer wg.Done(); _, _ = io.Copy(c, up) }()
			wg.Wait()
		}()
	}
}
