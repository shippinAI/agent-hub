package runner

import (
	"context"
	"crypto/sha256"
	"devagotchi/bridge/sandbox"
	"devagotchi/bridge/scope"
	"devagotchi/bridge/wire"
	"encoding/hex"
	"encoding/json"
	"os/exec"
	"strings"
)

func sha(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func (r *Runner) exec(ctx context.Context, f wire.Frame) wire.Result {
	var a execArgs
	_ = json.Unmarshal(f.Args, &a)
	if len(a.Argv) == 0 {
		return fail("invalid", "argv is empty")
	}
	sel := scope.Sel(f.Grant.Selector)
	env := filterEnv(a.Env, sel)
	spec := sandbox.Spec{Level: f.Grant.Level, Paths: strsOf(sel, "paths"), Cwd: a.Cwd, Argv: a.Argv, Env: env, Self: r.Self}
	stop := r.startProxy(ctx, &spec, sel)
	if stop != nil {
		defer stop()
	}
	cmd, err := sandbox.Command(spec)
	if err != nil {
		return fail("sandbox_unavailable", err.Error())
	}
	cap := a.CapBytes
	if cap <= 0 {
		cap = 1 << 20
	}
	var out, errb capped
	out.max, errb.max = cap, cap
	cmd.Stdout, cmd.Stderr = &out, &errb
	if a.Stdin != "" {
		cmd.Stdin = strings.NewReader(a.Stdin)
	}
	if err := cmd.Start(); err != nil {
		return fail("exec_failed", r.Redact.String(err.Error()))
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-ctx.Done():
		sandbox.Kill(cmd)
		<-done
		return wire.Result{OK: false, Error: "grant_revoked", Message: "call stopped: " + ctx.Err().Error(),
			Stdout: r.Redact.String(out.String()), Stderr: r.Redact.String(errb.String())}
	case err := <-done:
		code := exitCode(err)
		return wire.Result{OK: true, ExitCode: &code, Stdout: r.Redact.String(out.String()), Stderr: r.Redact.String(errb.String()),
			Truncated: out.truncated || errb.truncated, OutSHA256: sha([]byte(out.String()))}
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if ok := asExit(err, &ee); ok {
		return ee.ExitCode()
	}
	return -1
}
