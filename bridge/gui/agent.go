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

// helper holds the one live backend shared across the short-lived service connections.
type helper struct {
	ctx context.Context
	log func(string)
	mu  sync.Mutex
	b   Backend
}

func (h *helper) stop() {
	h.mu.Lock()
	if h.b != nil {
		h.b.Close()
		h.b = nil
	}
	h.mu.Unlock()
}

// Serve listens on an owner-only socket and services gui.* ops against a single shared backend. gui.observe
// opens it; gui.session_end or the panic hotkey closes it (one GUI session per machine, invariant 01).
func Serve(ctx context.Context, dir string, log func(string)) error {
	sock := SocketPath(dir)
	_ = os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		return err
	}
	_ = os.Chmod(sock, 0o600)
	defer os.Remove(sock)
	h := &helper{ctx: ctx, log: log}
	go func() { <-ctx.Done(); ln.Close(); h.stop() }()
	log("gui helper listening on " + sock)
	for {
		c, err := ln.Accept()
		if err != nil {
			return nil
		}
		go h.serveConn(c)
	}
}

func (h *helper) serveConn(c net.Conn) {
	defer c.Close()
	sc := bufio.NewScanner(c)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	enc := json.NewEncoder(c)
	for sc.Scan() {
		var r req
		if json.Unmarshal(sc.Bytes(), &r) != nil {
			continue
		}
		_ = enc.Encode(h.handle(r))
	}
}

func (h *helper) handle(r req) resp {
	h.mu.Lock()
	if r.Op == "gui.session" && h.b == nil {
		nb, err := Open(h.stop)
		if err != nil {
			h.mu.Unlock()
			return resp{Error: err.Error()}
		}
		h.b = nb
	}
	cur := h.b
	h.mu.Unlock()
	if r.Op == "gui.session_end" {
		h.stop()
		return resp{OK: true}
	}
	if cur == nil {
		return resp{Error: "no active gui session"}
	}
	data, err := Handle(h.ctx, cur, r.Op, r.Args)
	if err != nil {
		return resp{Error: err.Error()}
	}
	return resp{OK: true, Data: data}
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
