package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"agenthub/internal/store"
)

type leaseBody struct {
	Fence      int64           `json:"fence"`
	TTLSeconds int             `json:"ttl_seconds"`
	Reason     string          `json:"reason"`
	Summary    string          `json:"summary"`
	Note       string          `json:"note"`
	To         string          `json:"to"`
	Progress   json.RawMessage `json:"progress"`
	DependsOn  int64           `json:"depends_on"`
}

func (s *Server) taskRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/me", s.authed(s.me))
	mux.HandleFunc("GET /api/v1/tasks", s.authed(func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		q := r.URL.Query()
		holder := q.Get("holder")
		if holder == "me" {
			holder = p.ID
		}
		ts, err := s.St.Tasks(r.Context(), store.TaskFilter{Project: q.Get("project"), State: q.Get("state"), Holder: holder,
			Label: q.Get("label"), Query: q.Get("q"), ReadyOnly: q.Get("ready") == "1", Limit: int(qInt(r, "limit"))})
		return map[string]any{"tasks": ts, "server_now": nowUTC(), "head_seq": s.Broker.Head()}, err
	}))
	mux.HandleFunc("POST /api/v1/tasks", s.authed(func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		var in store.NewTask
		rid, err := decode(r, &in)
		if err != nil {
			return nil, err
		}
		return s.St.CreateTask(r.Context(), p, rid, in)
	}))
	mux.HandleFunc("GET /api/v1/tasks/{id}", s.authed(s.taskDetail))
	mux.HandleFunc("PATCH /api/v1/tasks/{id}", s.authed(func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		id, err := pathID(r)
		if err != nil {
			return nil, err
		}
		var in store.TaskPatch
		rid, err := decode(r, &in)
		if err != nil {
			return nil, err
		}
		if m := strings.Trim(r.Header.Get("If-Match"), `W/"`); m != "" {
			in.Version, _ = strconv.ParseInt(m, 10, 64)
		}
		return s.St.UpdateTask(r.Context(), p, rid, id, in)
	}))
	mux.HandleFunc("POST /api/v1/claim-next", s.authed(func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		var f store.NextFilter
		rid, err := decode(r, &f)
		if err != nil {
			return nil, err
		}
		return s.St.ClaimNext(r.Context(), p, rid, f)
	}))
	mux.HandleFunc("POST /api/v1/tasks/{id}/{op}", s.authed(s.taskOp))
}

func (s *Server) taskOp(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	if r.PathValue("op") == "notes" {
		var in store.NoteInput
		rid, err := decode(r, &in)
		if err != nil {
			return nil, err
		}
		return s.St.AddNote(ctx, p, rid, id, in)
	}
	var b leaseBody
	rid, err := decode(r, &b)
	if err != nil {
		return nil, err
	}
	switch r.PathValue("op") {
	case "claim":
		return s.St.Claim(ctx, p, rid, id, b.TTLSeconds)
	case "heartbeat":
		return s.St.Heartbeat(ctx, p, rid, id, b.Fence, b.Progress)
	case "release":
		return s.St.Release(ctx, p, rid, id, b.Fence, b.Reason)
	case "complete":
		return s.St.Complete(ctx, p, rid, id, b.Fence, b.Summary)
	case "handoff":
		return s.St.Handoff(ctx, p, rid, id, b.Fence, b.Note, b.To)
	case "force-release":
		return s.St.ForceRelease(ctx, p, rid, id, b.Reason)
	case "deps":
		return s.St.AddDep(ctx, p, rid, id, b.DependsOn)
	}
	return nil, &store.Error{Code: "not_found", Status: 404, Message: "unknown task operation"}
}

func (s *Server) taskDetail(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	t, err := s.St.Task(r.Context(), id)
	if err != nil {
		return nil, err
	}
	notes, err := s.St.Notes(r.Context(), id)
	if err != nil {
		return nil, err
	}
	deps, err := s.St.Deps(r.Context(), id)
	if err != nil {
		return nil, err
	}
	ev, err := s.St.Events(r.Context(), store.EventFilter{TaskID: id, Limit: 200})
	return map[string]any{"task": t, "notes": notes, "depends_on": deps, "events": ev, "server_now": nowUTC()}, err
}
