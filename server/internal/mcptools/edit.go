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

type evidenceIn struct {
	TaskID int64 `json:"task_id"`
	store.EvidenceInput
	RequestID string `json:"request_id,omitempty"`
}

type linkIn struct {
	TaskID int64 `json:"task_id"`
	store.LinkInput
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
	tool(s, "add_evidence", "Attach proof to a task: kind (test_run, ci_check, eval, review, ...), status pass|fail, "+
		"summary, ref (URL or id), optional sha256 and data. Include fence while you hold it. Lanes may require passing "+
		"evidence for the current lease before complete is accepted (error evidence_missing lists what is needed).",
		func(ctx context.Context, p *store.Principal, in evidenceIn) (any, error) {
			return d.st.AddEvidence(ctx, p, in.RequestID, in.TaskID, in.EvidenceInput)
		})
	tool(s, "link_git", "Link a task to its git work: kind branch|commit|pr|check_run, repo owner/name, ref (branch name, "+
		"commit sha, PR number or check name), optional url and head_sha. Link your branch right after you push it: the "+
		"hub's GitHub webhook then records commits, the PR and check runs, and attaches each finished check as evidence "+
		"ci.<check name> for the branch's newest commit.",
		func(ctx context.Context, p *store.Principal, in linkIn) (any, error) {
			return d.st.LinkGit(ctx, p, in.RequestID, in.TaskID, in.LinkInput)
		})
}
