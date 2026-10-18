package tools

import (
	"context"
	"fmt"
	"strings"

	"devagotchi/bridge/policy"
	"devagotchi/bridge/procs"
)

type readOutIn struct {
	PID           int  `json:"pid"`
	TimeoutMS     int  `json:"timeout_ms,omitempty" jsonschema:"offset 0: wait this long for new output (default default_timeout_ms)"`
	Offset        int  `json:"offset,omitempty" jsonschema:"0 = new lines since last read; >0 absolute line; <0 last N lines"`
	Length        int  `json:"length,omitempty" jsonschema:"max lines (default file_read_line_limit)"`
	VerboseTiming bool `json:"verbose_timing,omitempty"`
}

func readProcessOutput() Tool {
	return def(Tool{Name: "read_process_output", Title: "Read Process Output", Cap: policy.CapProc, ReadOnly: true,
		Desc: "Read a session's output by line window (offset 0 = new since the last read and waits up to timeout_ms for " +
			"some; positive = absolute line; negative = tail). Reports state (running / waiting_for_input / finished)."},
		func(ctx context.Context, e *Env, in readOutIn) Out {
			s, err := session(e, in.PID)
			if err != nil {
				return errOut(err)
			}
			if in.Length <= 0 {
				in.Length = e.Gate.Limits().FileReadLineLimit
			}
			wait, note := timeout(e, in.TimeoutMS)
			r := s.ReadLines(in.Offset, in.Length)
			if in.Offset == 0 && r.Returned == 0 && s.ExitCode() == nil {
				s.Wait(ctx, s.Output().Seq(), wait, e.Procs.Prompts, true, true)
				r = s.ReadLines(0, in.Length)
			}
			st := s.State(e.Procs.Prompts)
			head := fmt.Sprintf("%s[Reading %d lines from line %d (total: %d lines, %d remaining); state: %s]\n", note, r.Returned, r.From, r.Total, r.Remaining, st)
			return ok(head+e.Redact.String(r.Text), map[string]any{"pid": in.PID, "state": st, "exit_code": s.ExitCode(), "window": r})
		})
}

type interactIn struct {
	PID           int    `json:"pid"`
	Input         string `json:"input"`
	TimeoutMS     int    `json:"timeout_ms,omitempty" jsonschema:"max wait for the reply (default 8000)"`
	WaitForPrompt *bool  `json:"wait_for_prompt,omitempty" jsonschema:"return when a prompt appears (default true); false = return at the first settled output"`
	Raw           bool   `json:"raw,omitempty" jsonschema:"send input exactly (no newline added); use for control keys like \u0003"`
	VerboseTiming bool   `json:"verbose_timing,omitempty"`
}

func interactWithProcess() Tool {
	return def(Tool{Name: "interact_with_process", Title: "Send Input to Process", Cap: policy.CapProc, Destructive: true, OpenWorld: true,
		Desc: "Send input to a session (REPL, shell, database console) and return the output it produced, waiting for the " +
			"next prompt, exit, or timeout_ms. Input goes through the same grant check as start_process (DC #552)."},
		func(ctx context.Context, e *Env, in interactIn) Out {
			s, err := session(e, in.PID)
			if err != nil {
				return errOut(err)
			}
			if in.TimeoutMS <= 0 {
				in.TimeoutMS = 8000
			}
			wait, note := timeout(e, in.TimeoutMS)
			seq := s.Output().Seq()
			if err := s.Write(in.Input, in.Raw); err != nil {
				return fail("not_running", "the process has exited; start a new one")
			}
			wfp := truthy(in.WaitForPrompt, true)
			snap := s.Wait(ctx, seq, wait, e.Procs.Prompts, wfp, !wfp)
			s.MarkRead()
			if s.PTY {
				snap.Output = strings.TrimPrefix(strings.TrimPrefix(snap.Output, strings.TrimRight(in.Input, "\n")), "\r\n")
			}
			return procOut(e, note, s, snap, wait, in.VerboseTiming)
		})
}

func session(e *Env, pid int) (*procs.Session, error) {
	if err := e.Gate.Need(policy.CapProc); err != nil {
		return nil, err
	}
	s, ok := e.Procs.Get(pid)
	if !ok {
		return nil, &policy.Denied{Code: "not_found", Msg: fmt.Sprintf("no session with pid %d here (list_sessions shows them)", pid)}
	}
	return s, nil
}

