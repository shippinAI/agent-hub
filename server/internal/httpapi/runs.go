package httpapi

import (
	"net/http"
	"strconv"

	"agenthub/internal/store"
)

// runRoutes: POST /tasks/{id}/runs (in tasks.go) opens a run; these append to, finish and read it.
func (s *Server) runRoutes(mux *http.ServeMux) {
	runID := func(r *http.Request) (int64, error) {
		id, err := strconv.ParseInt(r.PathValue("run"), 10, 64)
		if err != nil {
			return 0, &store.Error{Code: "invalid", Status: 400, Message: "run id must be a number"}
		}
		return id, nil
	}
	mux.HandleFunc("POST /api/v1/runs/{run}/transcript", s.authed(func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		id, err := runID(r)
		if err != nil {
			return nil, err
		}
		var c store.RunChunk
		rid, err := decode(r, &c)
		if err != nil {
			return nil, err
		}
		return s.St.AppendTranscript(r.Context(), p, rid, id, c)
	}))
	mux.HandleFunc("POST /api/v1/runs/{run}/finish", s.authed(func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		id, err := runID(r)
		if err != nil {
			return nil, err
		}
		var in store.RunFinish
		rid, err := decode(r, &in)
		if err != nil {
			return nil, err
		}
		return s.St.FinishRun(r.Context(), p, rid, id, in)
	}))
	mux.HandleFunc("GET /api/v1/runs/{run}", s.authed(func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		id, err := runID(r)
		if err != nil {
			return nil, err
		}
		after := -1
		if v := r.URL.Query().Get("after_seq"); v != "" {
			if after, err = strconv.Atoi(v); err != nil {
				return nil, &store.Error{Code: "invalid", Status: 400, Message: "after_seq must be a number"}
			}
		}
		run, chunks, more, err := s.St.RunTranscript(r.Context(), id, after)
		return map[string]any{"run": run, "chunks": chunks, "more": more}, err
	}))
}
