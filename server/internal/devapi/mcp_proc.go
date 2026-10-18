package devapi

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"agenthub/internal/mcptools"
	"agenthub/internal/store"
)

type procStartIn struct {
	Machine string   `json:"machine"`
	Argv    []string `json:"argv"`
	Cwd     string   `json:"cwd,omitempty"`
	TTL     int      `json:"ttl_s,omitempty" jsonschema:"lease seconds (default 300, max 3600); heartbeat before it ends or the process is killed"`
	TaskID  *int64   `json:"task_id,omitempty"`
}

type procIn struct {
	LeaseID  string `json:"lease_id"`
	Fence    int64  `json:"fence"`
	Data     string `json:"data,omitempty" jsonschema:"proc_write: bytes to send to stdin"`
	AfterSeq int64  `json:"after_seq,omitempty" jsonschema:"proc_read: output after this sequence number"`
}

func (a *API) procTools(s *mcp.Server) {
	mcptools.Tool(s, "proc_start", "Start a long-running process under a proc.interactive grant; returns lease_id and fence.",
		func(ctx context.Context, p *store.Principal, in procStartIn) (any, error) {
			return a.Ops.ProcStart(ctx, p, in.Machine, in.Argv, in.Cwd, in.TTL, in.TaskID)
		})
	for _, n := range []string{"read", "write", "heartbeat", "kill"} {
		op := "proc." + n
		mcptools.Tool(s, "proc_"+n, "Process lease op "+n+" (lease_id + fence from proc_start).", func(ctx context.Context, p *store.Principal, in procIn) (any, error) {
			return a.Ops.ProcOp(ctx, p, in.LeaseID, in.Fence, op, map[string]any{"data": in.Data, "after_seq": in.AfterSeq})
		})
	}
}
