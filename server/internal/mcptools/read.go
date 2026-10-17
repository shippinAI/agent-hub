package mcptools

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"agenthub/internal/store"
)

type empty struct{}

type listIn struct {
	Project string `json:"project,omitempty" jsonschema:"project id filter"`
	State   string `json:"state,omitempty" jsonschema:"workflow state filter"`
	Holder  string `json:"holder,omitempty" jsonschema:"holder principal id, or 'me'"`
	Label   string `json:"label,omitempty" jsonschema:"label filter"`
	Limit   int    `json:"limit,omitempty" jsonschema:"max results (default 50)"`
}

type idIn struct {
	TaskID int64 `json:"task_id" jsonschema:"task id"`
}

type searchIn struct {
	Query string `json:"query" jsonschema:"words to find in titles, descriptions and notes"`
	Limit int    `json:"limit,omitempty"`
}

type waitIn struct {
	AfterSeq       int64 `json:"after_seq" jsonschema:"return events with seq greater than this"`
	TimeoutSeconds int   `json:"timeout_seconds,omitempty" jsonschema:"max wait, up to 25"`
}

func limit(n int) int {
	if n <= 0 || n > 200 {
		return 50
	}
	return n
}

func (d deps) readTools(s *mcp.Server) {
	tool(s, "whoami", "Your principal (from the token), roles and the hub's lease settings.",
		func(ctx context.Context, p *store.Principal, _ empty) (any, error) {
			st, err := d.st.SettingsMap(ctx)
			return map[string]any{"principal": p, "lease_ttl_seconds": st.Int("lease_ttl_seconds", 900),
				"heartbeat_every_seconds": st.Int("heartbeat_hint_seconds", 300), "server_now": time.Now().UTC()}, err
		})
	tool(s, "list_ready", "Tasks you could claim now (queue state, dependencies done, unheld), best first.",
		func(ctx context.Context, p *store.Principal, in listIn) (any, error) {
			ts, err := d.st.Tasks(ctx, store.TaskFilter{Project: in.Project, Label: in.Label, ReadyOnly: true, Limit: limit(in.Limit)})
			return map[string]any{"tasks": ts}, err
		})
	tool(s, "list_tasks", "List tasks with optional filters (holder 'me' = your claims).",
		func(ctx context.Context, p *store.Principal, in listIn) (any, error) {
			if in.Holder == "me" {
				in.Holder = p.ID
			}
			ts, err := d.st.Tasks(ctx, store.TaskFilter{Project: in.Project, State: in.State, Holder: in.Holder, Label: in.Label, Limit: limit(in.Limit)})
			return map[string]any{"tasks": ts}, err
		})
	tool(s, "get_task", "One task with its notes (including handoffs), dependencies and recent events.",
		func(ctx context.Context, p *store.Principal, in idIn) (any, error) {
			t, err := d.st.Task(ctx, in.TaskID)
			if err != nil {
				return nil, err
			}
			notes, _ := d.st.Notes(ctx, in.TaskID)
			deps, _ := d.st.Deps(ctx, in.TaskID)
			ev, err := d.st.Events(ctx, store.EventFilter{TaskID: in.TaskID, Limit: 50, Desc: true})
			return map[string]any{"task": t, "notes": notes, "depends_on": deps, "recent_events": ev}, err
		})
	tool(s, "search", "Full-text search over task titles, descriptions and notes.",
		func(ctx context.Context, p *store.Principal, in searchIn) (any, error) {
			ts, err := d.st.Tasks(ctx, store.TaskFilter{Query: in.Query, Limit: limit(in.Limit)})
			return map[string]any{"tasks": ts}, err
		})
	tool(s, "wait_for_updates", "Long-poll the event log: returns events after after_seq (waits up to timeout_seconds).",
		func(ctx context.Context, p *store.Principal, in waitIn) (any, error) {
			t := in.TimeoutSeconds
			if t <= 0 || t > 25 {
				t = 25
			}
			head := d.br.Wait(ctx, in.AfterSeq, time.Duration(t)*time.Second)
			ev, err := d.st.Events(ctx, store.EventFilter{AfterSeq: in.AfterSeq, Limit: 200})
			return map[string]any{"events": ev, "head_seq": head}, err
		})
}
