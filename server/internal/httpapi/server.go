// Package httpapi serves REST (/api/v1), SSE, the MCP endpoint and the embedded UI.
package httpapi

import (
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"

	"agenthub/internal/auth"
	"agenthub/internal/config"
	"agenthub/internal/store"
	"agenthub/internal/stream"
)

//go:embed all:static
var static embed.FS

// Server bundles the dependencies of every handler.
type Server struct {
	St     *store.Store
	Auth   *auth.Resolver
	Broker *stream.Broker
	Cfg    config.Config
	MCP    http.Handler
}

type handler func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error)

// Handler builds the full mux.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.taskRoutes(mux)
	s.adminRoutes(mux)
	s.eventRoutes(mux)
	if s.MCP != nil {
		mux.Handle("/mcp", s.MCP)
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { js(w, 200, map[string]bool{"ok": true}) })
	sub, _ := fs.Sub(static, "static")
	files := http.FileServer(http.FS(sub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fs.Stat(sub, strings.TrimPrefix(r.URL.Path, "/")); err != nil || r.URL.Path == "/" {
			r.URL.Path = "/"
		}
		files.ServeHTTP(w, r)
	})
	return secHeaders(mux)
}

func secHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:")
		h.ServeHTTP(w, r)
	})
}

// authed resolves the principal; cookie-less browser writes also need X-Hub-Client (CSRF guard).
func (s *Server) authed(h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, _, err := s.Auth.Identify(r)
		if err != nil {
			js(w, 401, map[string]any{"error": map[string]string{"code": "unauthenticated", "message": err.Error()}})
			return
		}
		if r.Method != "GET" && auth.Bearer(r) == "" && r.Header.Get("X-Hub-Client") == "" {
			js(w, 403, map[string]any{"error": map[string]string{"code": "csrf", "message": "missing X-Hub-Client header"}})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		out, err := h(w, r, p)
		if err != nil {
			writeErr(w, err)
			return
		}
		if store.ReplayOf(out) {
			w.Header().Set("Idempotent-Replayed", "true")
		}
		js(w, 200, out)
	}
}

func writeErr(w http.ResponseWriter, err error) {
	if e, ok := store.AsError(err); ok {
		js(w, e.Status, map[string]any{"error": e})
		return
	}
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		js(w, 413, map[string]any{"error": map[string]string{"code": "too_large", "message": "body over 1 MiB"}})
		return
	}
	log.Printf("internal error: %v", err)
	js(w, 500, map[string]any{"error": map[string]string{"code": "internal", "message": "internal error (see server log)"}})
}

func js(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// decode reads a JSON body into v (empty body is fine) and returns the request id.
func decode(r *http.Request, v any) (string, error) {
	var wrap struct {
		RequestID string `json:"request_id"`
	}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	if len(strings.TrimSpace(string(b))) > 0 {
		if err := json.Unmarshal(b, v); err != nil {
			return "", &store.Error{Code: "invalid", Status: 400, Message: "bad JSON: " + err.Error()}
		}
		_ = json.Unmarshal(b, &wrap)
	}
	if k := r.Header.Get("Idempotency-Key"); k != "" {
		return k, nil
	}
	return wrap.RequestID, nil
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return 0, &store.Error{Code: "invalid", Status: 400, Message: "task id must be a number"}
	}
	return id, nil
}

func qInt(r *http.Request, k string) int64 {
	v, _ := strconv.ParseInt(r.URL.Query().Get(k), 10, 64)
	return v
}
