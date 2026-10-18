// Package stream fans Postgres NOTIFY (hub_events, hub_actions) out to SSE clients and long-pollers.
package stream

import (
	"context"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Broker tracks the newest event seq and wakes subscribers when it moves.
type Broker struct {
	mu      sync.Mutex
	subs    map[chan int64]struct{}
	head    int64
	channel string
	headSQL string
}

// New returns an idle broker for the governance event log; call Run.
func New() *Broker { return NewFor("hub_events", `SELECT coalesce(max(seq),0) FROM events`) }

// NewFor returns a broker on another NOTIFY channel whose payload is an increasing id (the action feed).
func NewFor(channel, headSQL string) *Broker {
	return &Broker{subs: map[chan int64]struct{}{}, channel: channel, headSQL: headSQL}
}

// Run LISTENs on hub_events until ctx ends, reconnecting on errors.
func (b *Broker) Run(ctx context.Context, pool *pgxpool.Pool) {
	for ctx.Err() == nil {
		if err := b.listen(ctx, pool); err != nil && ctx.Err() == nil {
			log.Printf("stream: %v (reconnecting)", err)
			time.Sleep(time.Second)
		}
	}
}

func (b *Broker) listen(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "LISTEN "+b.channel); err != nil {
		return err
	}
	var head int64
	_ = conn.QueryRow(ctx, b.headSQL).Scan(&head)
	b.publish(head)
	for {
		n, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		if seq, err := strconv.ParseInt(n.Payload, 10, 64); err == nil {
			b.publish(seq)
		}
	}
}

func (b *Broker) publish(seq int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if seq > b.head {
		b.head = seq
	}
	for c := range b.subs {
		select {
		case c <- b.head:
		default:
		}
	}
}

// Head is the newest seq seen.
func (b *Broker) Head() int64 { b.mu.Lock(); defer b.mu.Unlock(); return b.head }

// Subscribe returns a channel that receives the head seq on change, and a cancel func.
func (b *Broker) Subscribe() (<-chan int64, func()) {
	c := make(chan int64, 1)
	b.mu.Lock()
	b.subs[c] = struct{}{}
	b.mu.Unlock()
	return c, func() { b.mu.Lock(); delete(b.subs, c); b.mu.Unlock() }
}

// Wait blocks until head > after or the timeout passes; returns the head.
func (b *Broker) Wait(ctx context.Context, after int64, d time.Duration) int64 {
	c, cancel := b.Subscribe()
	defer cancel()
	if h := b.Head(); h > after {
		return h
	}
	t := time.NewTimer(d)
	defer t.Stop()
	for {
		select {
		case h := <-c:
			if h > after {
				return h
			}
		case <-t.C:
			return b.Head()
		case <-ctx.Done():
			return b.Head()
		}
	}
}
