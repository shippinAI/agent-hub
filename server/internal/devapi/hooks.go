package devapi

import (
	"encoding/json"
	"time"

	"agenthub/internal/access"
	"agenthub/internal/guidekit"
	"agenthub/internal/guides"
	"devagotchi/bridge/scope"
)

// guideHooks connects guide runs to machine ops and the access flow. Steps never create grants: a
// request_access step files a normal request that a human approves (01 invariant 9).
type guideHooks struct{ a *API }

func (h guideHooks) Exec(c guides.Ctx, machine string, s guidekit.Step) (string, any, error) {
	switch s.Op {
	case "probe":
		r, err := h.a.Ops.Probe(c, machine)
		if err != nil {
			return "fail", map[string]string{"error": err.Error()}, nil
		}
		return map[bool]string{true: "ok", false: "fail"}[r.OK], r, nil
	case "gui_ready":
		m, err := h.a.Machines.Get(c, machine)
		if err != nil {
			return "", nil, err
		}
		var ready map[string]string
		_ = json.Unmarshal(m.GUIReady, &ready)
		ok := len(ready) > 0
		for _, v := range ready {
			ok = ok && v == "granted"
		}
		return map[bool]string{true: "ok", false: "fail"}[ok], ready, nil
	}
	var argv []string
	if l, ok := s.Args["argv"].([]any); ok {
		for _, x := range l {
			argv = append(argv, x.(string))
		}
	}
	t := 120
	if v, ok := s.Args["timeout_s"].(float64); ok {
		t = int(v)
	}
	op, action := "exec", "exec"
	if s.Op != "exec" {
		op = map[string]string{"fs_read": "fs.read", "fs_list": "fs.list", "fs_stat": "fs.stat", "fs_write": "fs.write"}[s.Op]
		action = scope.CapabilityFor(op)
	}
	args := map[string]any{"argv": argv, "timeout_s": t}
	for k, v := range s.Args {
		args[k] = v
	}
	r, err := h.a.Ops.Call(c, c.P, machine, action, op, args, scope.Call{Argv: argv}, time.Duration(t+10)*time.Second)
	if err != nil {
		return "fail", map[string]string{"error": err.Error()}, nil
	}
	want := 0
	if s.ExpectExit != nil {
		want = *s.ExpectExit
	}
	ok := r.OK && (r.ExitCode == nil || *r.ExitCode == want)
	return map[bool]string{true: "ok", false: "fail"}[ok], map[string]any{"exit_code": r.ExitCode, "stdout": tail(r.Stdout), "stderr": tail(r.Stderr)}, nil
}

func (h guideHooks) Request(c guides.Ctx, run guides.Run, s guidekit.Step) (string, any, error) {
	var group string
	_ = h.a.St.DB.QueryRow(c, `SELECT group_id::text FROM access_requests WHERE guide_run_id::text=$1 AND requester=$2 AND reason=$3
		ORDER BY created_at DESC LIMIT 1`, run.ID, c.P.ID, s.Reason).Scan(&group)
	if group == "" {
		out, err := h.a.Access.Request(c, c.P, "", access.Req{Machine: deref(run.MachineID), Preset: s.Preset, Capability: s.Capability, Selector: s.Selector,
			SandboxLevel: s.SandboxLevel, Duration: s.Duration, Reason: s.Reason, TaskID: run.TaskID, GuideRunID: run.ID})
		if err != nil {
			return "", nil, err
		}
		return "pending", out, nil
	}
	sts, err := h.a.Access.Lookup(c, group)
	if err != nil {
		return "", nil, err
	}
	switch sum := access.Summary(sts); sum {
	case "approved", "pending":
		return sum, sts, nil
	}
	return "denied", sts, nil
}

func tail(s string) string {
	if len(s) > 2000 {
		return s[len(s)-2000:]
	}
	return s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
