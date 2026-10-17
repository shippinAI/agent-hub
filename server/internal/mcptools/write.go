package mcptools

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"agenthub/internal/store"
)

type claimIn struct {
	TaskID     int64  `json:"task_id"`
	TTLSeconds int    `json:"ttl_seconds,omitempty" jsonschema:"lease length; clamped to the hub's min/max"`
	RequestID  string `json:"request_id,omitempty" jsonschema:"idempotency key; reuse it when retrying"`
}

type nextIn struct {
	store.NextFilter
	RequestID string `json:"request_id,omitempty" jsonschema:"idempotency key; reuse it when retrying"`
}

type beatIn struct {
	TaskID    int64          `json:"task_id"`
	Fence     int64          `json:"fence" jsonschema:"fence from your claim"`
	Progress  map[string]any `json:"progress,omitempty" jsonschema:"optional progress details kept on the task"`
	RequestID string         `json:"request_id,omitempty"`
}

type endIn struct {
	TaskID    int64  `json:"task_id"`
	Fence     int64  `json:"fence" jsonschema:"fence from your claim"`
	Text      string `json:"text,omitempty" jsonschema:"release reason / completion summary"`
	RequestID string `json:"request_id,omitempty"`
}

type handoffIn struct {
	TaskID    int64  `json:"task_id"`
	Fence     int64  `json:"fence"`
	Note      string `json:"note" jsonschema:"what is done, what is next, gotchas"`
	To        string `json:"to,omitempty" jsonschema:"principal id to reserve the task for"`
	RequestID string `json:"request_id,omitempty"`
}

func (d deps) leaseTools(s *mcp.Server) {
	tool(s, "claim", "Lease one task. Returns fence + lease_expires_at; send the fence on every later write.",
		func(ctx context.Context, p *store.Principal, in claimIn) (any, error) {
			return d.st.Claim(ctx, p, in.RequestID, in.TaskID, in.TTLSeconds)
		})
	tool(s, "claim_next", "Atomically lease the best ready task you may take (optional project/labels/states filter).",
		func(ctx context.Context, p *store.Principal, in nextIn) (any, error) {
			return d.st.ClaimNext(ctx, p, in.RequestID, in.NextFilter)
		})
	tool(s, "heartbeat", "Renew your lease (call every heartbeat_every_seconds). Fails if you lost the task.",
		func(ctx context.Context, p *store.Principal, in beatIn) (any, error) {
			var prog json.RawMessage
			if in.Progress != nil {
				prog, _ = json.Marshal(in.Progress)
			}
			return d.st.Heartbeat(ctx, p, in.RequestID, in.TaskID, in.Fence, prog)
		})
	tool(s, "release", "Give a task back unfinished (text = reason).",
		func(ctx context.Context, p *store.Principal, in endIn) (any, error) {
			return d.st.Release(ctx, p, in.RequestID, in.TaskID, in.Fence, in.Text)
		})
	tool(s, "complete", "Finish your step (text = summary: what changed, how verified). Moves to the lane's next state.",
		func(ctx context.Context, p *store.Principal, in endIn) (any, error) {
			return d.st.Complete(ctx, p, in.RequestID, in.TaskID, in.Fence, in.Text)
		})
	tool(s, "handoff", "Stop and hand over: writes a handoff note, releases the lease, optionally reserves it for 'to'.",
		func(ctx context.Context, p *store.Principal, in handoffIn) (any, error) {
			return d.st.Handoff(ctx, p, in.RequestID, in.TaskID, in.Fence, in.Note, in.To)
		})
}
