// Package runner executes call frames on the bridge: verify the signature and deadline, recheck the grant
// against the hub if the cache is stale, build the sandbox from the grant scope, run, and stream the result.
package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"devagotchi/bridge/config"
	"devagotchi/bridge/fsops"
	"devagotchi/bridge/hubclient"
	"devagotchi/bridge/redact"
	"devagotchi/bridge/sandbox"
	"devagotchi/bridge/scope"
	"devagotchi/bridge/wire"
)

// Runner runs calls for one machine.
type Runner struct {
	Cfg       config.Config
	Client    *hubclient.Client
	Key       []byte
	Self      string
	ConfigDir string
	Redact    redact.Redactor
	Log       func(string)
	mu        sync.Mutex
	running   map[string]context.CancelFunc
	cache     map[string]cacheEntry
}

type cacheEntry struct {
	live    bool
	version int64
	at      time.Time
}

// New builds a runner.
func New(cfg config.Config, c *hubclient.Client, key []byte, self string) *Runner {
	return &Runner{Cfg: cfg, Client: c, Key: key, Self: self, Redact: redact.New(cfg.SecretPatterns),
		Log: func(string) {}, running: map[string]context.CancelFunc{}, cache: map[string]cacheEntry{}}
}

// Handle dispatches one inbound frame.
func (r *Runner) Handle(ctx context.Context, f wire.Frame) {
	switch f.Type {
	case "call":
		go r.runCall(ctx, f)
	case "revoke":
		r.cancelGrant(f.GrantID)
	case "cancel":
		r.cancelCall(f.CallID)
	}
}

func (r *Runner) cancelCall(id string) {
	r.mu.Lock()
	if cancel := r.running[id]; cancel != nil {
		cancel()
	}
	r.mu.Unlock()
}

func (r *Runner) cancelGrant(id string) {
	r.mu.Lock()
	r.cache[id] = cacheEntry{live: false, at: time.Now()}
	for cid, cancel := range r.running {
		if strings.HasPrefix(cid, id+"|") {
			cancel()
		}
	}
	r.mu.Unlock()
}

func (r *Runner) send(ctx context.Context, f wire.Frame) {
	if err := r.Client.Send(ctx, f); err != nil {
		r.Log(r.Redact.String("send result: " + err.Error()))
	}
}

func (r *Runner) result(ctx context.Context, callID string, res wire.Result) {
	r.send(ctx, wire.Frame{V: wire.Version, Type: "result", CallID: callID, Result: &res})
}

func fail(code, msg string) wire.Result { return wire.Result{OK: false, Error: code, Message: msg} }

// grantLive checks the grant cache, refetching from the hub when older than grant_cache_seconds.
func (r *Runner) grantLive(ctx context.Context, f wire.Frame) bool {
	if f.GrantID == "" || f.GrantID == "probe" || f.GrantID == "operator" {
		return true
	}
	r.mu.Lock()
	e, ok := r.cache[f.GrantID]
	r.mu.Unlock()
	if ok && time.Since(e.at) < time.Duration(r.Cfg.CacheSeconds)*time.Second {
		return e.live && e.version >= f.GrantVersion
	}
	live, ver, err := r.Client.GrantLive(ctx, f.GrantID)
	if err != nil {
		return ok && e.live
	}
	r.mu.Lock()
	r.cache[f.GrantID] = cacheEntry{live: live, version: ver, at: time.Now()}
	r.mu.Unlock()
	return live && ver >= f.GrantVersion
}

func (r *Runner) runCall(ctx context.Context, f wire.Frame) {
	if err := wire.Verify(f, r.Key, time.Now()); err != nil {
		r.result(ctx, f.CallID, fail("bad_signature", err.Error()))
		return
	}
	if !r.grantLive(ctx, f) {
		r.result(ctx, f.CallID, fail("grant_revoked", "the hub says this grant is no longer live"))
		return
	}
	cctx, cancel := context.WithDeadline(ctx, f.Deadline)
	defer cancel()
	r.mu.Lock()
	r.running[f.GrantID+"|"+f.CallID] = cancel
	r.running[f.CallID] = cancel
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.running, f.GrantID+"|"+f.CallID)
		delete(r.running, f.CallID)
		r.mu.Unlock()
	}()
	r.send(ctx, wire.Frame{V: wire.Version, Type: "ack", CallID: f.CallID})
	start := time.Now()
	var res wire.Result
	switch {
	case f.Op == "probe":
		res = wire.Result{OK: true, Data: mustJSON(map[string]string{"sandbox": sandbox.Kind(), "host": hostname()})}
	case f.Op == "exec":
		res = r.exec(cctx, f)
	case strings.HasPrefix(f.Op, "fs."):
		res = r.fs(cctx, f)
	case strings.HasPrefix(f.Op, "gui."):
		res = r.guiOp(cctx, f)
	default:
		res = fail("unsupported_op", "this bridge build does not handle "+f.Op)
	}
	res.DurationMS = time.Since(start).Milliseconds()
	r.result(ctx, f.CallID, res)
}

func mustJSON(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

func hostname() string { h, _ := os.Hostname(); return h }

// execArgs is the exec op payload.
type execArgs struct {
	Argv     []string          `json:"argv"`
	Cwd      string            `json:"cwd"`
	Env      map[string]string `json:"env"`
	TimeoutS int               `json:"timeout_s"`
	Stdin    string            `json:"stdin"`
	CapBytes int               `json:"cap_bytes"`
}

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

func (r *Runner) fs(ctx context.Context, f wire.Frame) wire.Result {
	var op fsops.Op
	_ = json.Unmarshal(f.Args, &op)
	op.Kind = strings.TrimPrefix(f.Op, "fs.")
	sel := scope.Sel(f.Grant.Selector)
	paths := strsOf(sel, "paths")
	for _, p := range append([]string{op.Path}, op.To) {
		if p != "" && !pathAllowed(p, paths) {
			return fail("grant_required", fmt.Sprintf("path %s is outside the grant", p))
		}
	}
	res := fsops.DoSandboxed(ctx, r.Self, f.Grant.Level, paths, op)
	if res.Error != "" {
		return wire.Result{OK: false, Error: res.Error, Message: res.Message, Data: fsops.Marshal(res)}
	}
	return wire.Result{OK: true, Data: fsops.Marshal(res), OutSHA256: res.SHA256}
}

func pathAllowed(p string, paths []string) bool {
	if len(paths) == 0 {
		return true
	}
	for _, a := range paths {
		if scope.Within(p, a) {
			return true
		}
	}
	return false
}

func filterEnv(env map[string]string, sel scope.Sel) map[string]string {
	allow, ok := sel.Strs("env_allow")
	if !ok {
		return map[string]string{}
	}
	out := map[string]string{}
	for _, k := range allow {
		if v, ok := env[k]; ok {
			out[k] = v
		} else if v := os.Getenv(k); v != "" {
			out[k] = v
		}
	}
	return out
}

func strsOf(sel scope.Sel, k string) []string {
	s, _ := sel.Strs(k)
	return s
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
