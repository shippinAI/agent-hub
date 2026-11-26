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

type usageIn struct {
	store.UsageInput
	RequestID string `json:"request_id,omitempty"`
}

type usageSumIn struct {
	GroupBy string `json:"group_by,omitempty" jsonschema:"task, principal, project, model, provider or day"`
	TaskID  int64  `json:"task_id,omitempty"`
	Mine    bool   `json:"mine,omitempty" jsonschema:"only your own usage"`
}

type scoresIn struct {
	GroupBy string   `json:"group_by,omitempty" jsonschema:"principal (default), adapter, model, lane or project"`
	Project string   `json:"project,omitempty"`
	Lane    string   `json:"lane,omitempty" jsonschema:"active state, e.g. in_progress or reviewing"`
	Labels  []string `json:"labels,omitempty"`
	Mine    bool     `json:"mine,omitempty" jsonschema:"only your own runs"`
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
	tool(s, "record_usage", "Record one model call or turn you made: provider_id, model, billing_kind api_key|subscription|router, "+
		"tokens by kind (cache_read is part of input, reasoning part of output), source, and task_id plus fence when it was for a "+
		"task. Give cost_micros with cost_basis provider_charged or client_estimate if your client reports cost; otherwise the hub "+
		"prices it from its catalog. external_ref (turn or response id) makes a retry count once. The reply lists budgets in scope; "+
		"a used-up budget stops new claims (budget_exceeded).",
		func(ctx context.Context, p *store.Principal, in usageIn) (any, error) {
			return d.st.RecordUsage(ctx, p, in.RequestID, in.UsageInput)
		})
	tool(s, "usage_summary", "Usage and cost totals (cost in micro-USD), grouped by task, principal, project, model, provider or day, "+
		"plus every budget with its spend this period, so you can check headroom before a long task.",
		func(ctx context.Context, p *store.Principal, in usageSumIn) (any, error) {
			f := store.UsageFilter{GroupBy: in.GroupBy, TaskID: in.TaskID}
			if in.Mine {
				f.Principal = p.ID
			}
			totals, err := d.st.Usage(ctx, f)
			if err != nil {
				return nil, err
			}
			b, err := d.st.Budgets(ctx)
			return map[string]any{"totals": totals, "budgets": b}, err
		})
	tool(s, "run_scores", "How agent runs turned out, grouped by principal, adapter, model, lane or project: accepted (a later "+
		"lane such as review finished the task), reworked (the same lane had to run again), pending, handed off, failed or lost, "+
		"with success rate and cost per accepted run. When routing is on, claim_next skips lanes where your success rate is low.",
		func(ctx context.Context, p *store.Principal, in scoresIn) (any, error) {
			f := store.ScoreFilter{GroupBy: in.GroupBy, Project: in.Project, Lane: in.Lane, Labels: in.Labels}
			if in.Mine {
				f.Principal = p.ID
			}
			sc, err := d.st.Scores(ctx, f)
			return map[string]any{"scores": sc}, err
		})
	tool(s, "link_git", "Link a task to its git work: kind branch|commit|pr|check_run, repo owner/name, ref (branch name, "+
		"commit sha, PR number or check name), optional url and head_sha. Link your branch right after you push it: the "+
		"hub's GitHub webhook then records commits, the PR and check runs, and attaches each finished check as evidence "+
		"ci.<check name> for the branch's newest commit.",
		func(ctx context.Context, p *store.Principal, in linkIn) (any, error) {
			return d.st.LinkGit(ctx, p, in.RequestID, in.TaskID, in.LinkInput)
		})
}
