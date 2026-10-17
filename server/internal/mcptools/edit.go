package mcptools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"agenthub/internal/store"
)

type createIn struct {
	store.NewTask
	RequestID string `json:"request_id,omitempty"`
}

type updateIn struct {
	TaskID int64 `json:"task_id"`
	store.TaskPatch
	RequestID string `json:"request_id,omitempty"`
}

type noteIn struct {
	TaskID int64 `json:"task_id"`
	store.NoteInput
	RequestID string `json:"request_id,omitempty"`
}

func (d deps) editTools(s *mcp.Server) {
	tool(s, "create_task", "Create a task in a project (starts in the workflow's initial state).",
		func(ctx context.Context, p *store.Principal, in createIn) (any, error) {
			return d.st.CreateTask(ctx, p, in.RequestID, in.NewTask)
		})
	tool(s, "update", "Edit a task. Needs version (optimistic concurrency) and, while you hold it, fence.",
		func(ctx context.Context, p *store.Principal, in updateIn) (any, error) {
			return d.st.UpdateTask(ctx, p, in.RequestID, in.TaskID, in.TaskPatch)
		})
	tool(s, "add_note", "Append a note (kind note|decision|blocker|...) to a task; include fence if you hold it.",
		func(ctx context.Context, p *store.Principal, in noteIn) (any, error) {
			return d.st.AddNote(ctx, p, in.RequestID, in.TaskID, in.NoteInput)
		})
}
