package devapi

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"agenthub/internal/mcptools"
	"agenthub/internal/store"
	"devagotchi/bridge/scope"
)

type execIn struct {
	Machine  string            `json:"machine"`
	Argv     []string          `json:"argv" jsonschema:"command and arguments (compared element-wise with the grant's argv_prefix)"`
	Cwd      string            `json:"cwd,omitempty"`
	Env      map[string]string `json:"env,omitempty" jsonschema:"only names in the grant's env_allow pass"`
	TimeoutS int               `json:"timeout_s,omitempty" jsonschema:"default 120, max exec_timeout_max_seconds (preset 1800)"`
	Stdin    string            `json:"stdin,omitempty"`
	Level    string            `json:"sandbox_level,omitempty" jsonschema:"optional: run at a narrower level than the grant"`
	Ticket   string            `json:"ticket,omitempty" jsonschema:"change ticket id when the policy needs one (argv must equal the ticket's command)"`
}

type fsIn struct {
	Machine      string `json:"machine"`
	Path         string `json:"path"`
	To           string `json:"to,omitempty" jsonschema:"fs_move target"`
	Content      string `json:"content,omitempty"`
	ExpectSHA256 string `json:"expect_sha256,omitempty"`
	ExpectAbsent bool   `json:"expect_absent,omitempty"`
	Offset       int64  `json:"offset,omitempty"`
	Limit        int64  `json:"limit,omitempty" jsonschema:"bytes, max 1 MiB per read"`
	Edits        []any  `json:"edits,omitempty" jsonschema:"fs_edit: [{old,new,count?}] each old must match exactly count times"`
}

func (a *API) opsTools(s *mcp.Server) {
	mcptools.Tool(s, "exec", "Run one command on a machine inside the grant's sandbox. Output is capped at exec_output_cap_bytes (truncated flag); timeout default 120 s, max exec_timeout_max_seconds.",
		func(ctx context.Context, p *store.Principal, in execIn) (any, error) {
			set, _ := a.St.LoadSettings(ctx)
			t := in.TimeoutS
			if t <= 0 {
				t = 120
			}
			t = min(t, set.Int("exec_timeout_max_seconds", 1800))
			c := scope.Call{Argv: in.Argv, Level: in.Level, Ticket: in.Ticket}
			if in.Cwd != "" {
				c.Paths = []string{in.Cwd}
			}
			args := map[string]any{"argv": in.Argv, "cwd": in.Cwd, "env": in.Env, "timeout_s": t, "stdin": in.Stdin, "cap_bytes": set.Int("exec_output_cap_bytes", 1<<20)}
			return a.Ops.Call(ctx, p, in.Machine, "exec", "exec", args, c, time.Duration(t+10)*time.Second)
		})
	for _, op := range []struct{ name, op, action, desc string }{
		{"fs_read", "fs.read", "fs.read", "Read a file (offset/limit, max 1 MiB) -> content or base64, sha256, size, mtime."},
		{"fs_list", "fs.list", "fs.read", "List a directory."},
		{"fs_stat", "fs.stat", "fs.read", "Stat a path."},
		{"fs_write", "fs.write", "fs.write", "Write a file atomically (temp+fsync+rename). Needs expect_sha256 of the current file or expect_absent; mismatch -> hash_mismatch with the current hash."},
		{"fs_edit", "fs.edit", "fs.write", "Apply exact string edits; every old must match exactly count (default 1) times or nothing is written (edit_no_match / edit_ambiguous)."},
		{"fs_move", "fs.move", "fs.write", "Move a file (expect_sha256 of the source)."},
		{"fs_delete", "fs.delete", "fs.write", "Delete a file; needs expect_sha256."},
	} {
		op := op
		mcptools.Tool(s, op.name, op.desc, func(ctx context.Context, p *store.Principal, in fsIn) (any, error) {
			paths := []string{in.Path}
			if in.To != "" {
				paths = append(paths, in.To)
			}
			return a.Ops.Call(ctx, p, in.Machine, op.action, op.op, in, scope.Call{Paths: paths}, 60*time.Second)
		})
	}
	a.procTools(s)
	a.machineToolTools(s)
}
