// Package runner executes call frames on the bridge: verify the signature and deadline, recheck the grant
// against the hub if the cache is stale, build the sandbox from the grant scope, run, and stream the result.
package runner

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"

	"devagotchi/bridge/config"
	"devagotchi/bridge/hubclient"
	"devagotchi/bridge/redact"
	"devagotchi/bridge/sandbox"
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

func (r *Runner) send(ctx context.Context, f wire.Frame) {
	if err := r.Client.Send(ctx, f); err != nil {
		r.Log(r.Redact.String("send result: " + err.Error()))
	}
}

func (r *Runner) result(ctx context.Context, callID string, res wire.Result) {
	r.send(ctx, wire.Frame{V: wire.Version, Type: "result", CallID: callID, Result: &res})
}

func fail(code, msg string) wire.Result { return wire.Result{OK: false, Error: code, Message: msg} }

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
