package tools

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"devagotchi/bridge/policy"
	"devagotchi/bridge/sandbox"
)

type runCommandIn struct {
	Argv      []string `json:"argv,omitempty" jsonschema:"argument list run directly (preferred)"`
	Command   string   `json:"command,omitempty" jsonschema:"command line for the shell profile"`
	Shell     string   `json:"shell,omitempty"`
	Cwd       string   `json:"cwd,omitempty"`
	TimeoutMS int      `json:"timeout_ms,omitempty" jsonschema:"kill after this long (default default_timeout_ms, capped by max_timeout_ms; both stated in the result)"`
	Stdin     string   `json:"stdin,omitempty"`
}

func runCommand() Tool {
	return def(Tool{Name: "run_command", Title: "Run Command (one-shot)", Cap: policy.CapExec, Destructive: true, OpenWorld: true,
		Desc: "Run one command to completion inside the sandbox and return exit code, stdout and stderr (each capped at " +
			"output_buffer_bytes, flagged when cut). If it outlives timeout_ms it is killed and the result says timed_out."},
		func(ctx context.Context, e *Env, in runCommandIn) Out {
			if err := e.Gate.Need(policy.CapExec); err != nil {
				return errOut(err)
			}
			argv, _, err := argvFor(e.Gate.P, in.Argv, in.Command, in.Shell)
			if err != nil {
				return fail("invalid", "%v", err)
			}
			return runOnce(ctx, e, argv, in.Cwd, in.Stdin, in.TimeoutMS)
		})
}

type runCodeIn struct {
	Language  string `json:"language" jsonschema:"code runner from the policy: python, node, r, ..."`
	Code      string `json:"code"`
	TimeoutMS int    `json:"timeout_ms,omitempty"`
}

func runCode() Tool {
	return def(Tool{Name: "run_code", Title: "Run Code in Memory", Cap: policy.CapExec, Destructive: true,
		Desc: "Run a snippet with a policy code runner (python -c, node -e, Rscript -e) inside the sandbox, without " +
			"writing a file; stateless, like DC's node:local. For a stateful REPL use start_process with python3 -i."},
		func(ctx context.Context, e *Env, in runCodeIn) Out {
			if err := e.Gate.Need(policy.CapExec); err != nil {
				return errOut(err)
			}
			prof, err := profile(e.Gate.P.CodeRunners, in.Language, "")
			if err != nil {
				return fail("invalid", "%v", err)
			}
			return runOnce(ctx, e, append(append([]string{}, prof.Argv...), in.Code), "", "", in.TimeoutMS)
		})
}

func runOnce(ctx context.Context, e *Env, argv []string, cwd, stdin string, ms int) Out {
	cmd, stop, err := e.confined(argv, cwd, false)
	if err != nil {
		return errOut(err)
	}
	defer stop()
	limit := e.Gate.Limits().OutputBufferBytes
	out, errb := &capBuf{max: limit}, &capBuf{max: limit}
	cmd.Stdout, cmd.Stderr = out, errb
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	wait, note := timeout(e, ms)
	start := time.Now()
	if err := cmd.Start(); err != nil {
		return fail("exec_failed", "%s", e.Redact.String(err.Error()))
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	timedOut := false
	select {
	case <-done:
	case <-time.After(wait):
		timedOut = true
	case <-ctx.Done():
		timedOut = true
	}
	if timedOut {
		sandbox.Kill(cmd)
		<-done
	}
	code := cmd.ProcessState.ExitCode()
	data := map[string]any{"exit_code": code, "timed_out": timedOut, "duration_ms": time.Since(start).Milliseconds(),
		"timeout_ms_applied": wait.Milliseconds(), "stdout_truncated": out.cut, "stderr_truncated": errb.cut}
	head := fmt.Sprintf("%sexit code %d in %d ms", note, code, time.Since(start).Milliseconds())
	if timedOut {
		head += fmt.Sprintf(" (killed: exceeded timeout %d ms)", wait.Milliseconds())
	}
	text := head + "\n--- stdout ---\n" + e.Redact.String(out.String()) + "\n--- stderr ---\n" + e.Redact.String(errb.String())
	res := ok(text, data)
	if timedOut {
		res.Code = "timeout"
	}
	return res
}

// capBuf keeps at most max bytes and flags truncation; safe for concurrent writers.
type capBuf struct {
	mu  sync.Mutex
	b   []byte
	max int
	cut bool
}

func (c *capBuf) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := len(p)
	room := c.max - len(c.b)
	if room < len(p) {
		c.cut = true
		p = p[:max(0, room)]
	}
	c.b = append(c.b, p...)
	return n, nil
}

func (c *capBuf) String() string { c.mu.Lock(); defer c.mu.Unlock(); return string(c.b) }
