package devapi

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"agenthub/internal/auth"
	"agenthub/internal/httpapi"
	"agenthub/internal/machines"
	"agenthub/internal/store"
	"devagotchi/bridge/wire"
)

// bridgeRoutes are for machine principals only (06 "Bridge endpoints").
func (a *API) bridgeRoutes(mux *http.ServeMux) {
	lim := &limiter{hits: map[string][]time.Time{}}
	mux.HandleFunc("POST /bridge/v1/enroll", func(w http.ResponseWriter, r *http.Request) {
		if !lim.ok(r.RemoteAddr, 10, time.Minute) {
			httpapi.WriteErr(w, store.Fail(429, "rate_limited", "too many enrollment attempts; wait a minute"))
			return
		}
		var in machines.EnrollIn
		if _, err := httpapi.Decode(r, &in); err != nil {
			httpapi.WriteErr(w, err)
			return
		}
		out, err := a.Machines.Enroll(r.Context(), in)
		if err != nil {
			httpapi.WriteErr(w, err)
			return
		}
		httpapi.JSON(w, 200, out)
	})
	mux.HandleFunc("POST /bridge/v1/poll", a.machine(func(w http.ResponseWriter, r *http.Request, m machines.Machine) (any, error) {
		set, _ := a.St.LoadSettings(r.Context())
		return a.Hub.Next(r.Context(), m.ID, time.Duration(set.Int("bridge_poll_seconds", 25))*time.Second, 16), nil
	}))
	mux.HandleFunc("POST /bridge/v1/result", a.machine(func(w http.ResponseWriter, r *http.Request, m machines.Machine) (any, error) {
		var frames []wire.Frame
		if _, err := httpapi.Decode(r, &frames); err != nil {
			return nil, err
		}
		for _, f := range frames {
			a.inbound(r.Context(), m, f)
		}
		return map[string]int{"accepted": len(frames)}, nil
	}))
	mux.HandleFunc("GET /bridge/v1/grant/{id}", a.machine(func(w http.ResponseWriter, r *http.Request, m machines.Machine) (any, error) {
		var live bool
		var ver int64
		err := a.St.DB.QueryRow(r.Context(), `SELECT ended_at IS NULL AND NOT paused AND (expires_at IS NULL OR expires_at>now()), version FROM grants
			WHERE id::text=$1 AND machine_id=$2`, r.PathValue("id"), m.ID).Scan(&live, &ver)
		if err != nil {
			return map[string]any{"live": false}, nil
		}
		return map[string]any{"live": live && !m.Disabled, "version": ver}, nil
	}))
	mux.HandleFunc("POST /bridge/v1/gui-ready", a.machine(func(w http.ResponseWriter, r *http.Request, m machines.Machine) (any, error) {
		var in struct {
			Ready   map[string]string `json:"gui_ready"`
			Display string            `json:"display_server"`
		}
		if _, err := httpapi.Decode(r, &in); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, a.Machines.GUIReady(r.Context(), m.ID, in.Ready, in.Display)
	}))
	mux.HandleFunc("GET /bridge/v1", func(w http.ResponseWriter, r *http.Request) {
		m, err := a.machineOf(r)
		if err != nil {
			httpapi.WriteErr(w, err)
			return
		}
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		ws.SetReadLimit(8 << 20)
		a.Hub.Serve(r.Context(), m.ID, ws)
	})
}

// inbound handles one bridge frame: call frames go to the router, gui_stop pauses GUI on that machine.
func (a *API) inbound(ctx context.Context, m machines.Machine, f wire.Frame) {
	if f.Type != "gui_stop" {
		a.Hub.Deliver(m.ID, f)
		return
	}
	_, _ = a.St.Write(ctx, store.System, "", "gui_stop", m.ID, func(w *store.W) (any, error) {
		if _, err := w.Exec(`UPDATE machines SET gui_paused=true, version=version+1 WHERE id=$1`, m.ID); err != nil {
			return nil, err
		}
		if _, err := w.Exec(`UPDATE gui_sessions SET state='ended', ended_at=now(), end_reason='operator_stop' WHERE machine_id=$1 AND state<>'ended'`, m.ID); err != nil {
			return nil, err
		}
		return nil, w.Event("gui.stopped", nil, nil, map[string]any{"machine": m.ID, "via": firstOr(f.Data, "hotkey")})
	})
}

func (a *API) machineOf(r *http.Request) (machines.Machine, error) {
	p, err := a.Srv.Auth.Token(r.Context(), auth.Bearer(r))
	if err != nil || p.Kind != "machine" {
		return machines.Machine{}, store.Fail(401, "unauthenticated", "a machine token is required")
	}
	return a.Machines.ByPrincipal(r.Context(), p.ID)
}

func (a *API) machine(fn func(w http.ResponseWriter, r *http.Request, m machines.Machine) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m, err := a.machineOf(r)
		if err != nil {
			httpapi.WriteErr(w, err)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16<<20)
		out, err := fn(w, r, m)
		if err != nil {
			httpapi.WriteErr(w, err)
			return
		}
		b, _ := json.Marshal(out)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}
}

type limiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func (l *limiter) ok(key string, n int, per time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if i := len(key) - 1; i > 0 {
		for i > 0 && key[i] != ':' {
			i--
		}
		key = key[:i]
	}
	keep := []time.Time{}
	for _, t := range l.hits[key] {
		if time.Since(t) < per {
			keep = append(keep, t)
		}
	}
	l.hits[key] = append(keep, time.Now())
	return len(keep) < n
}
