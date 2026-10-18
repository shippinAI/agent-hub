// Package bridgehub routes signed calls to connected bridges and collects their results. Bridges reach it
// by long-poll (POST /bridge/v1/poll + /result) or a WebSocket; both feed the same queues.
package bridgehub

import (
	"context"
	"strings"
	"sync"
	"time"

	"devagotchi/bridge/wire"
)

// Hub is the in-memory router (one per hub process).
type Hub struct {
	mu      sync.Mutex
	conns   map[string]*conn
	pending map[string]*call
	// Seen is told when a bridge polls or its socket is up (presence: connected only, never online).
	Seen func(machineID string, connected bool)
}

type conn struct {
	out      chan wire.Frame
	lastSeen time.Time
	sockets  int
}

type call struct {
	machine string
	done    chan wire.Result
	stdout  strings.Builder
	stderr  strings.Builder
	acked   bool
}

// New returns an empty hub.
func New() *Hub { return &Hub{conns: map[string]*conn{}, pending: map[string]*call{}} }

func (h *Hub) conn(m string) *conn {
	c, ok := h.conns[m]
	if !ok {
		c = &conn{out: make(chan wire.Frame, 256)}
		h.conns[m] = c
	}
	return c
}

// Connected reports whether the bridge polled within window or holds a socket.
func (h *Hub) Connected(m string, window time.Duration) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	c, ok := h.conns[m]
	return ok && (c.sockets > 0 || time.Since(c.lastSeen) < window)
}

// Call queues a signed call frame and waits for its result (or ctx / the frame deadline).
func (h *Hub) Call(ctx context.Context, m string, f wire.Frame) (wire.Result, error) {
	cl := &call{machine: m, done: make(chan wire.Result, 1)}
	h.mu.Lock()
	h.pending[f.CallID] = cl
	c := h.conn(m)
	h.mu.Unlock()
	defer func() { h.mu.Lock(); delete(h.pending, f.CallID); h.mu.Unlock() }()
	select {
	case c.out <- f:
	default:
		return wire.Result{}, ErrBusy
	}
	ctx, cancel := context.WithDeadline(ctx, f.Deadline.Add(5*time.Second))
	defer cancel()
	select {
	case r := <-cl.done:
		return r, nil
	case <-ctx.Done():
		h.Push(m, wire.Frame{V: wire.Version, Type: "cancel", CallID: f.CallID})
		return wire.Result{}, ErrTimeout
	}
}

// Push queues a control frame (revoke, cancel, ping) without waiting.
func (h *Hub) Push(m string, f wire.Frame) {
	h.mu.Lock()
	c := h.conn(m)
	h.mu.Unlock()
	select {
	case c.out <- f:
	default:
	}
}

// Next waits up to wait for frames for machine m (long-poll), returning at most max.
func (h *Hub) Next(ctx context.Context, m string, wait time.Duration, max int) []wire.Frame {
	h.mu.Lock()
	c := h.conn(m)
	c.lastSeen = time.Now()
	h.mu.Unlock()
	if h.Seen != nil {
		h.Seen(m, true)
	}
	out := []wire.Frame{}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case f := <-c.out:
		out = append(out, f)
	case <-t.C:
		return out
	case <-ctx.Done():
		return out
	}
	for len(out) < max {
		select {
		case f := <-c.out:
			out = append(out, f)
		default:
			return out
		}
	}
	return out
}

// Deliver takes a frame from bridge m: ack, chunk or result for one of its pending calls.
func (h *Hub) Deliver(m string, f wire.Frame) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c, ok := h.conns[m]; ok {
		c.lastSeen = time.Now()
	}
	cl, ok := h.pending[f.CallID]
	if !ok || cl.machine != m {
		return
	}
	switch f.Type {
	case "ack":
		cl.acked = true
	case "chunk":
		if f.Stream == "stderr" {
			cl.stderr.WriteString(f.Data)
		} else {
			cl.stdout.WriteString(f.Data)
		}
	case "result":
		if f.Result == nil {
			return
		}
		r := *f.Result
		if r.Stdout == "" {
			r.Stdout = cl.stdout.String()
		}
		if r.Stderr == "" {
			r.Stderr = cl.stderr.String()
		}
		select {
		case cl.done <- r:
		default:
		}
	}
}
