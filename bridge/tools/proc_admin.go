package tools

import (
	"context"
	"fmt"
	"strings"

	"devagotchi/bridge/policy"
	"devagotchi/bridge/procs"
)

type pidIn struct {
	PID int `json:"pid"`
}

func forceTerminate() Tool {
	return def(Tool{Name: "force_terminate", Title: "Force Terminate Process", Cap: policy.CapProc, Destructive: true,
		Desc: "Kill a session started by this server, including its whole process tree (process group / Job Object)."},
		func(ctx context.Context, e *Env, in pidIn) Out {
			s, err := session(e, in.PID)
			if err != nil {
				return errOut(err)
			}
			_ = s.Kill()
			return ok(fmt.Sprintf("Terminated session %d (exit %v)", in.PID, deref(s.ExitCode())), map[string]any{"pid": in.PID, "exit_code": s.ExitCode()})
		})
}

func listSessions() Tool {
	return def(Tool{Name: "list_sessions", Title: "List Terminal Sessions", Cap: policy.CapProc, ReadOnly: true,
		Desc: "List this server's sessions: pid, command, state, blocked (waiting for input), runtime, exit code."},
		func(ctx context.Context, e *Env, _ noIn) Out {
			if err := e.Gate.Need(policy.CapProc); err != nil {
				return errOut(err)
			}
			l := e.Procs.List()
			var b strings.Builder
			for _, s := range l {
				fmt.Fprintf(&b, "PID %d  %s  blocked=%v  runtime=%ds  %s\n", s.PID, s.State, s.Blocked, s.RuntimeMS/1000, e.Redact.String(s.Command))
			}
			if len(l) == 0 {
				b.WriteString("No sessions.\n")
			}
			return ok(b.String(), map[string]any{"sessions": l})
		})
}

func listProcesses() Tool {
	return def(Tool{Name: "list_processes", Title: "List Running Processes", Cap: policy.CapProcList, ReadOnly: true,
		Desc: "List processes on this machine: pid, ppid, user, CPU %, RSS, command (CPU/RSS are not sampled on Windows)."},
		func(ctx context.Context, e *Env, _ noIn) Out {
			if err := e.Gate.Need(policy.CapProcList); err != nil {
				return errOut(err)
			}
			ps, err := procs.ListAll()
			if err != nil {
				return errOut(err)
			}
			var b strings.Builder
			fmt.Fprintf(&b, "%-8s %-8s %-10s %6s %10s  %s\n", "PID", "PPID", "USER", "CPU%", "RSS_KB", "COMMAND")
			for _, p := range ps {
				c := e.Redact.String(p.Command)
				if len(c) > 160 {
					c = c[:160] + "…"
				}
				fmt.Fprintf(&b, "%-8d %-8d %-10s %6.1f %10d  %s\n", p.PID, p.PPID, p.User, p.CPUPercent, p.RSSBytes/1024, c)
			}
			return ok(b.String(), map[string]any{"count": len(ps)})
		})
}

type killIn struct {
	PID   int  `json:"pid"`
	Force bool `json:"force,omitempty" jsonschema:"SIGKILL at once instead of SIGTERM then SIGKILL after 2 s"`
}

func killProcess() Tool {
	return def(Tool{Name: "kill_process", Title: "Kill Process", Cap: policy.CapProcKill, Destructive: true,
		Desc: "Kill a process by pid. Sessions started here need only proc.interactive; any other process needs the " +
			"proc.kill capability. pid <= 1, this server and its client are always refused."},
		func(ctx context.Context, e *Env, in killIn) Out {
			if in.PID <= 0 {
				return fail("invalid", "pid must be greater than 0")
			}
			if s, mine := e.Procs.Get(in.PID); mine && e.Gate.Has(policy.CapProc) {
				_ = s.Kill()
				return ok(fmt.Sprintf("Killed session %d", in.PID), map[string]any{"pid": in.PID, "session": true})
			}
			if err := e.Gate.Need(policy.CapProcKill); err != nil {
				return errOut(err)
			}
			if err := procs.KillPID(in.PID, in.Force); err != nil {
				return fail("kill_failed", "%v", err)
			}
			return ok(fmt.Sprintf("Killed process %d", in.PID), map[string]any{"pid": in.PID})
		})
}

func deref(p *int) any {
	if p == nil {
		return "running"
	}
	return *p
}
