package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"devagotchi/bridge/policy"
	"devagotchi/bridge/procs"
	"devagotchi/bridge/sandbox"
)

type startProcessIn struct {
	Command       string   `json:"command,omitempty" jsonschema:"command line run by the shell profile (see shell)"`
	Argv          []string `json:"argv,omitempty" jsonschema:"argument list run directly, no shell parsing (preferred)"`
	TimeoutMS     int      `json:"timeout_ms,omitempty" jsonschema:"how long to wait for output/prompt/exit before returning (default default_timeout_ms; the process keeps running after)"`
	Shell         string   `json:"shell,omitempty" jsonschema:"shell profile name from the policy (sh, bash, zsh, powershell, pwsh, cmd, *-pty)"`
	Cwd           string   `json:"cwd,omitempty"`
	PTY           *bool    `json:"pty,omitempty" jsonschema:"run on a pseudo-terminal (ConPTY on Windows) for REPLs and console apps"`
	VerboseTiming bool     `json:"verbose_timing,omitempty"`
}

func startProcess() Tool {
	return def(Tool{Name: "start_process", Title: "Start Terminal Process", Cap: policy.CapProc, Destructive: true, OpenWorld: true,
		Desc: "Start a process inside the OS sandbox (bubblewrap / Seatbelt; ConPTY for console apps on Windows) and wait " +
			"until it exits, shows a prompt (it is then waiting_for_input), or timeout_ms passes. A timeout never kills it: the " +
			"result says how long it waited, and read_process_output / interact_with_process continue. No command blocklist " +
			"to trip over: the sandbox limits what any command can touch. Use argv to skip shell parsing entirely."},
		func(ctx context.Context, e *Env, in startProcessIn) Out {
			if err := e.Gate.Need(policy.CapProc); err != nil {
				return errOut(err)
			}
			argv, pty, err := argvFor(e.Gate.P, in.Argv, in.Command, in.Shell)
			if err != nil {
				return fail("invalid", "%v", err)
			}
			pty = truthy(in.PTY, pty)
			cmd, stop, err := e.confined(argv, in.Cwd, pty)
			if err != nil {
				return errOut(err)
			}
			label := in.Command
			if label == "" {
				label = strings.Join(in.Argv, " ")
			}
			s, err := e.Procs.Start(procs.Spec{Cmd: cmd, PTY: pty, Command: label})
			if err != nil {
				stop()
				return fail("exec_failed", "%s", e.Redact.String(err.Error()))
			}
			go func() { <-s.Done(); stop() }()
			wait, note := timeout(e, in.TimeoutMS)
			snap := s.Wait(ctx, 0, wait, e.Procs.Prompts, true, false)
			s.MarkRead()
			return procOut(e, fmt.Sprintf("Process started with PID %d (sandbox %s, level %s%s).\n%s", s.PID, sandbox.Kind(), e.Gate.Level(), map[bool]string{true: ", pty"}[pty], note), s, snap, wait, in.VerboseTiming)
		})
}

// procOut renders a session snapshot with its state and honest timing.
func procOut(e *Env, head string, s *procs.Session, snap procs.Snapshot, wait time.Duration, verbose bool) Out {
	state := map[string]string{procs.StateWaiting: "waiting for input (prompt shown)", procs.StateFinished: "finished",
		procs.StateRunning: "still running"}[snap.State]
	if snap.Reason == "timeout" {
		state += fmt.Sprintf(" (stopped waiting after %d ms; the process was not killed)", snap.WaitedMS)
	}
	if snap.ExitCode != nil {
		state += fmt.Sprintf(", exit code %d", *snap.ExitCode)
	}
	data := map[string]any{"pid": s.PID, "state": snap.State, "reason": snap.Reason, "exit_code": snap.ExitCode,
		"waited_ms": snap.WaitedMS, "timeout_ms_applied": wait.Milliseconds()}
	if verbose {
		data["timing"] = map[string]any{"started": s.Started.Format(time.RFC3339Nano), "exit_reason": snap.Reason, "seq": snap.Seq, "output_dropped": snap.Lost}
	}
	return ok(head+"State: "+state+"\n\n"+e.Redact.String(snap.Output), data)
}
