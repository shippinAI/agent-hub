package devapi

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"agenthub/internal/access"
	"agenthub/internal/mcptools"
	"agenthub/internal/store"
)

// Instructions is appended to the hub's MCP instructions.
const Instructions = `

Machines (Devagotchi): machines_list -> request_access {machine, capability|preset, selector, sandbox_level, duration, reason>=20 chars} ` +
	`-> wait_for_access -> exec / fs_* / proc_* / gui_*. A human approves every grant (maybe narrower); grant_required carries a request draft. ` +
	`Follow guides with guides_search, guide_run_start, guide_run_step. Never ask the operator for secrets.`

type machineIn struct {
	Machine string `json:"machine,omitempty" jsonschema:"machine id"`
}

type waitIn struct {
	RequestID string `json:"request_id" jsonschema:"request id or group id from request_access"`
	TimeoutS  int    `json:"timeout_s,omitempty" jsonschema:"long-poll up to 25 seconds"`
}

type routeIn struct {
	Role    string `json:"role"`
	ChatApp string `json:"chat_app,omitempty"`
}

func (a *API) accessTools(s *mcp.Server) {
	mcptools.Tool(s, "machines_list", "List machines with connected (bridge reachable) and online (last executed call succeeded recently), last_error, and your live grants.",
		func(ctx context.Context, p *store.Principal, _ struct{}) (any, error) {
			ms, err := a.Machines.List(ctx)
			if err != nil {
				return nil, err
			}
			mine, _ := a.Access.Grants(ctx, true, "", p.ID)
			return map[string]any{"machines": ms, "my_grants": mine}, nil
		})
	mcptools.Tool(s, "capabilities_list", "Capabilities you can request, each with its selector JSON Schema, offered sandbox levels and risk titles; plus grant presets and duration options.",
		func(ctx context.Context, p *store.Principal, _ struct{}) (any, error) {
			caps, err := a.Catalog.List(ctx, "capabilities")
			if err != nil {
				return nil, err
			}
			presets, _ := a.Catalog.List(ctx, "grant-presets")
			risks, _ := a.Catalog.List(ctx, "capability-risks")
			set, _ := a.St.LoadSettings(ctx)
			return map[string]any{"capabilities": caps, "presets": presets, "capability_risks": risks, "durations": set.Strings("grant_duration_options"),
				"default_preset": set.String("grant_default_preset", "")}, nil
		})
	mcptools.Tool(s, "request_access", "Ask the operator for access. reason (>=20 chars) is shown verbatim. preset expands to its items. Returns pending + request ids; then call wait_for_access. You can never create or widen a grant yourself.",
		func(ctx context.Context, p *store.Principal, in access.Req) (any, error) {
			return a.Access.Request(ctx, p, "", in)
		})
	mcptools.Tool(s, "wait_for_access", "Long-poll (<=25 s) a request or group until decided. Returns each item's status and the granted scope, which may be narrower than asked.",
		func(ctx context.Context, p *store.Principal, in waitIn) (any, error) {
			sts, err := a.Access.Wait(ctx, in.RequestID, time.Duration(in.TimeoutS)*time.Second)
			if err != nil {
				return nil, err
			}
			return map[string]any{"status": access.Summary(sts), "items": sts}, nil
		})
	mcptools.Tool(s, "my_grants", "Your live grants with expiry and uses (optionally for one machine).",
		func(ctx context.Context, p *store.Principal, in machineIn) (any, error) {
			return a.Access.Grants(ctx, true, in.Machine, p.ID)
		})
	mcptools.Tool(s, "role_route", "Which subscription and worker would take a task for this role right now (never a secret), with skipped options.",
		func(ctx context.Context, p *store.Principal, in routeIn) (any, error) {
			r, err := a.Subs.Resolve(ctx, in.Role, in.ChatApp)
			if err != nil {
				return nil, store.Fail(404, "no_route", "%v", err)
			}
			return r, nil
		})
}
