//go:build linux

package gui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
)

// SocketPath is the owner-only unix socket the service uses to reach the desktop helper.
func SocketPath(dir string) string { return filepath.Join(dir, "gui.sock") }

type req struct {
	Op   string          `json:"op"`
	Args json.RawMessage `json:"args"`
}
type resp struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

// Serve runs the helper: it listens on an owner-only socket and services one backend. Each gui.observe opens
// the backend, gui.session_end closes it; the panic hotkey calls onStop which closes the socket connection.
func Serve(ctx context.Context, dir string, log func(string)) error {
	sock := SocketPath(dir)
	_ = os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		return err
	}
	_ = os.Chmod(sock, 0o600)
	defer os.Remove(sock)
	go func() { <-ctx.Done(); ln.Close() }()
	log("gui helper listening on " + sock)
	for {
		c, err := ln.Accept()
		if err != nil {
			return nil
		}
		serveConn(ctx, c, log)
	}
}

func serveConn(ctx context.Context, c net.Conn, log func(string)) {
	defer c.Close()
	var (
		mu sync.Mutex
		b  Backend
	)
	stop := func() {
		mu.Lock()
		if b != nil {
			b.Close()
			b = nil
		}
		mu.Unlock()
		c.Close()
	}
	sc := bufio.NewScanner(c)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	enc := json.NewEncoder(c)
	for sc.Scan() {
		var r req
		if json.Unmarshal(sc.Bytes(), &r) != nil {
			continue
		}
		mu.Lock()
		if r.Op == "gui.observe" && b == nil {
			nb, err := Open(stop)
			if err != nil {
				mu.Unlock()
				_ = enc.Encode(resp{Error: err.Error()})
				continue
			}
			b = nb
		}
		cur := b
		mu.Unlock()
		if r.Op == "gui.session_end" {
			stop()
			_ = enc.Encode(resp{OK: true})
			return
		}
		if cur == nil {
			_ = enc.Encode(resp{Error: "no active gui session"})
			continue
		}
		data, err := Handle(ctx, cur, r.Op, r.Args)
		if err != nil {
			_ = enc.Encode(resp{Error: err.Error()})
			continue
		}
		_ = enc.Encode(resp{OK: true, Data: data})
	}
}

// Call connects to the helper socket and runs one op (used by the service). It returns the raw data payload.
func Call(dir, op string, args json.RawMessage) (json.RawMessage, error) {
	c, err := net.Dial("unix", SocketPath(dir))
	if err != nil {
		return nil, fmt.Errorf("gui helper not running: %w", err)
	}
	defer c.Close()
	b, _ := json.Marshal(req{Op: op, Args: args})
	if _, err := c.Write(append(b, '\n')); err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(c)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	if !sc.Scan() {
		return nil, fmt.Errorf("gui helper closed the connection")
	}
	var r resp
	if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
		return nil, err
	}
	if !r.OK {
		return nil, fmt.Errorf("%s", r.Error)
	}
	return r.Data, nil
}
