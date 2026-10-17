package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"agenthub/internal/store"
)

func nowUTC() time.Time { return time.Now().UTC() }

func (s *Server) eventRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/events", s.authed(func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		q := r.URL.Query()
		ev, err := s.St.Events(r.Context(), store.EventFilter{AfterSeq: qInt(r, "after_seq"), TaskID: qInt(r, "task_id"),
			Actor: q.Get("actor"), Action: q.Get("action"), Limit: int(qInt(r, "limit")), Desc: q.Get("order") == "desc"})
		return map[string]any{"events": ev, "head_seq": s.Broker.Head()}, err
	}))
	mux.HandleFunc("GET /api/v1/events/verify", s.authed(func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return s.St.VerifyChain(r.Context())
	}))
	mux.HandleFunc("GET /api/v1/protocol", s.authed(func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		st, err := s.St.SettingsMap(r.Context())
		if err != nil {
			return nil, err
		}
		return map[string]any{"agents_md": AgentsMD(s.Cfg.PublicURL, st.Int("heartbeat_hint_seconds", 300)),
			"connect": ConnectSnippets(s.Cfg.PublicURL, p.ID)}, nil
	}))
	mux.HandleFunc("GET /api/v1/stream", s.sse)
}

// sse streams events after Last-Event-ID / after_seq, then live ones as the broker announces them.
func (s *Server) sse(w http.ResponseWriter, r *http.Request) {
	if _, _, err := s.Auth.Identify(r); err != nil {
		js(w, 401, map[string]any{"error": map[string]string{"code": "unauthenticated", "message": err.Error()}})
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}
	after := qInt(r, "after_seq")
	if id, err := strconv.ParseInt(r.Header.Get("Last-Event-ID"), 10, 64); err == nil {
		after = id
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	ch, cancel := s.Broker.Subscribe()
	defer cancel()
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	fmt.Fprintf(w, "retry: 3000\n\n")
	for {
		for {
			ev, err := s.St.Events(r.Context(), store.EventFilter{AfterSeq: after, Limit: 500})
			if err != nil {
				return
			}
			for _, e := range ev {
				b, _ := json.Marshal(e)
				fmt.Fprintf(w, "id: %d\nevent: hub\ndata: %s\n\n", e.Seq, b)
				after = e.Seq
			}
			fl.Flush()
			if len(ev) < 500 {
				break
			}
		}
		select {
		case <-r.Context().Done():
			return
		case <-ch:
		case <-ping.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			fl.Flush()
		}
	}
}
