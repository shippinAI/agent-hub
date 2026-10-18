package runner

import (
	"context"
	"encoding/json"
	"time"

	"devagotchi/bridge/procs"
	"devagotchi/bridge/sandbox"
	"devagotchi/bridge/scope"
	"devagotchi/bridge/wire"
)

// procArgs is the payload of proc.start / read / write / heartbeat / kill (docs/devagotchi/03).
type procArgs struct {
	Argv     []string `json:"argv"`
	Cwd      string   `json:"cwd"`
	PTY      bool     `json:"pty"`
	Lease    string   `json:"lease_id"`
	Fence    int64    `json:"fence"`
	TTLS     int      `json:"ttl_s"`
	AfterSeq int64    `json:"after_seq"`
	Data     string   `json:"data"`
	WaitMS   int      `json:"wait_ms"`
}

func (r *Runner) proc(ctx context.Context, f wire.Frame) wire.Result {
	var a procArgs
	if err := json.Unmarshal(f.Args, &a); err != nil || a.Lease == "" {
		return fail("invalid", "proc ops need lease_id")
	}
	if f.Op == "proc.start" {
		return r.procStart(f, a)
	}
	s, ok := r.sessions().ByLease(a.Lease)
	if !ok {
		return fail("not_found", "no process for lease "+a.Lease+" on this machine")
	}
	if s.Fence != a.Fence && f.GrantID != "operator" {
		return fail("stale_fence", "fence does not match the lease holder")
	}
	switch f.Op {
	case "proc.write":
		if err := s.Write(a.Data, true); err != nil {
			return fail("not_running", "the process has exited")
		}
	case "proc.heartbeat":
		s.Heartbeat()
	case "proc.kill":
		_ = s.Kill()
	case "proc.read":
		if s.Output().Seq() <= a.AfterSeq && s.ExitCode() == nil {
			wait := time.Duration(min(max(a.WaitMS, 0), 20000)) * time.Millisecond
			s.Wait(ctx, a.AfterSeq, wait, r.sessions().Prompts, true, true)
		}
	default:
		return fail("unsupported_op", f.Op)
	}
	return r.procState(s, a.AfterSeq)
}

func (r *Runner) procStart(f wire.Frame, a procArgs) wire.Result {
	sel := scope.Sel(f.Grant.Selector)
	c := scope.Call{Op: "proc.start", Argv: a.Argv}
	if a.Cwd != "" {
		c.Paths = []string{a.Cwd}
	}
	if err := scope.Fits(f.Grant.Capability, f.Grant.Level, sel, c); err != nil || len(a.Argv) == 0 {
		return fail("scope_violation", "proc.start does not fit the grant")
	}
	if _, dup := r.sessions().ByLease(a.Lease); dup {
		return fail("exists", "lease already has a process")
	}
	spec := sandbox.Spec{Level: f.Grant.Level, Paths: strsOf(sel, "paths"), Cwd: a.Cwd, Argv: a.Argv, Env: filterEnv(nil, sel), Self: r.Self}
	if a.PTY {
		spec.Env["TERM"] = "xterm-256color"
	}
	stop := r.startProxy(context.Background(), &spec, sel)
	cmd, err := sandbox.Command(spec)
	if err != nil {
		if stop != nil {
			stop()
		}
		return fail("sandbox_unavailable", err.Error())
	}
	s, err := r.sessions().Start(procs.Spec{Cmd: cmd, PTY: a.PTY, Command: a.Argv[0], Lease: a.Lease, Fence: a.Fence,
		TTL: time.Duration(max(a.TTLS, 30)) * time.Second})
	if err != nil {
		if stop != nil {
			stop()
		}
		return fail("exec_failed", r.Redact.String(err.Error()))
	}
	go func() {
		<-s.Done()
		if stop != nil {
			stop()
		}
	}()
	s.Wait(context.Background(), 0, time.Second, r.sessions().Prompts, true, false)
	return r.procState(s, 0)
}

// procState returns output after `after` plus the session state; ExitCode is set once it finished.
func (r *Runner) procState(s *procs.Session, after int64) wire.Result {
	out, seq, lost := s.Output().Since(after)
	st := s.State(r.sessions().Prompts)
	return wire.Result{OK: true, ExitCode: s.ExitCode(), Stdout: r.Redact.String(out),
		Data: mustJSON(map[string]any{"pid": s.PID, "seq": seq, "lost": lost, "state": st})}
}
