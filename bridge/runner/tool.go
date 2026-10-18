package runner

import (
	"context"
	"encoding/json"

	"devagotchi/bridge/policy"
	"devagotchi/bridge/procs"
	"devagotchi/bridge/scope"
	"devagotchi/bridge/tools"
	"devagotchi/bridge/wire"
)

// sessions is the runner's process manager, shared by proc.* ops and the tool op.
func (r *Runner) sessions() *procs.Manager {
	r.toolsOnce.Do(func() {
		r.env = tools.NewEnv(policy.NewGate(policy.Defaults()), "hub", r.Self, nil, r.Log)
		r.env.Redact = r.Redact
	})
	return r.env.Procs
}

type toolArgs struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// tool runs any Desktop Commander-equivalent tool under a gate built from the signed grant only:
// capability = the grant's, roots = selector paths, hosts = selector hosts, level = the grant's.
func (r *Runner) tool(ctx context.Context, f wire.Frame) wire.Result {
	var a toolArgs
	if err := json.Unmarshal(f.Args, &a); err != nil {
		return fail("invalid", "tool op needs {name, arguments}")
	}
	t, ok := tools.Find(a.Name)
	if !ok {
		return fail("invalid", "unknown tool "+a.Name)
	}
	sel := scope.Sel(f.Grant.Selector)
	if err := prefixCheck(a, sel); err != nil {
		return fail("scope_violation", err.Error())
	}
	r.sessions()
	env := *r.env
	env.Gate = gateFor(f.Grant)
	out := t.Run(ctx, &env, a.Arguments)
	res := wire.Result{OK: out.Code == "", Error: out.Code, Stdout: out.Text, Data: mustJSON(out.Data)}
	if out.Code != "" {
		res.Message = out.Text
	}
	return res
}

func gateFor(g *wire.Grant) *policy.Gate {
	sel := scope.Sel(g.Selector)
	p := policy.Defaults()
	p.SandboxLevel, p.Capabilities = g.Level, []string{g.Capability}
	if g.Capability == policy.CapExec || g.Capability == policy.CapProc {
		p.Capabilities = append(p.Capabilities, policy.CapRead) // cwd checks; the process can read there anyway
	}
	p.Roots = strsOf(sel, "paths")
	p.URLHosts, p.EgressHosts, p.EnvAllow = strsOf(sel, "hosts"), strsOf(sel, "hosts"), strsOf(sel, "env_allow")
	gt := policy.NewGate(p)
	gt.AnyPath = len(p.Roots) == 0
	return gt
}

// prefixCheck applies the grant's command prefixes to process tools: with prefixes, only argv is accepted.
func prefixCheck(a toolArgs, sel scope.Sel) error {
	if len(sel.Prefixes()) == 0 {
		return nil
	}
	switch a.Name {
	case "start_process", "run_command", "run_code", "interact_with_process":
	default:
		return nil
	}
	var in struct {
		Argv []string `json:"argv"`
	}
	_ = json.Unmarshal(a.Arguments, &in)
	if a.Name == "interact_with_process" {
		return nil // input to a session that already passed this check at start
	}
	if len(in.Argv) == 0 || a.Name == "run_code" {
		return errPrefix
	}
	return scope.Fits("exec", "", sel, scope.Call{Op: "exec", Argv: in.Argv})
}

var errPrefix = &policy.Denied{Code: "scope_violation", Msg: "this grant limits commands to prefixes; pass argv (no shell string, no run_code)"}
