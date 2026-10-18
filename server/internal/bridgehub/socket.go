package bridgehub

import (
	"context"
	"encoding/json"
	"time"

	"github.com/coder/websocket"

	"devagotchi/bridge/wire"
)

// Serve runs one bridge WebSocket: frames out from the queue, frames in to Deliver, pings as presence.
func (h *Hub) Serve(ctx context.Context, m string, ws *websocket.Conn) {
	h.mu.Lock()
	c := h.conn(m)
	c.sockets++
	h.mu.Unlock()
	if h.Seen != nil {
		h.Seen(m, true)
	}
	defer func() {
		h.mu.Lock()
		c.sockets--
		c.lastSeen = time.Now()
		h.mu.Unlock()
		if h.Seen != nil {
			h.Seen(m, false)
		}
	}()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		defer cancel()
		for {
			_, b, err := ws.Read(ctx)
			if err != nil {
				return
			}
			var f wire.Frame
			if json.Unmarshal(b, &f) == nil {
				h.Deliver(m, f)
			}
		}
	}()
	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = ws.Close(websocket.StatusNormalClosure, "bye")
			return
		case f := <-c.out:
			b, _ := json.Marshal(f)
			if err := ws.Write(ctx, websocket.MessageText, b); err != nil {
				select {
				case c.out <- f:
				default:
				}
				return
			}
		case <-ping.C:
			if err := ws.Ping(ctx); err != nil {
				return
			}
		}
	}
}
