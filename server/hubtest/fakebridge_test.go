package hubtest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"devagotchi/bridge/wire"
)

// fakeBridge enrolls with a code and answers calls over long-poll, verifying every signature. "sleep"
// commands block until a revoke or cancel frame names their grant or call.
type fakeBridge struct {
	t       *testing.T
	url     string
	token   string
	machine string
	mu      sync.Mutex
	revoked map[string]chan struct{}
	calls   []wire.Frame
	badSig  int
}

func (e *env) enrollFake(code string) *fakeBridge {
	b := &fakeBridge{t: e.t, url: e.url, revoked: map[string]chan struct{}{}}
	var out struct {
		MachineID string `json:"machine_id"`
		Token     string `json:"token"`
	}
	if st := b.post("/bridge/v1/enroll", map[string]any{"code": code, "os": "linux", "arch": "amd64", "bridge_version": "test", "sandbox_kind": "bubblewrap"}, &out); st != 200 {
		e.t.Fatalf("enroll status %d", st)
	}
	b.token, b.machine = out.Token, out.MachineID
	ctx, cancel := context.WithCancel(context.Background())
	e.t.Cleanup(cancel)
	go b.loop(ctx)
	return b
}

func (b *fakeBridge) post(path string, in, out any) int {
	raw, _ := json.Marshal(in)
	req, _ := http.NewRequest("POST", b.url+path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if b.token != "" {
		req.Header.Set("Authorization", "Bearer "+b.token)
	}
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	defer r.Body.Close()
	if out != nil {
		_ = json.NewDecoder(r.Body).Decode(out)
	}
	return r.StatusCode
}

func (b *fakeBridge) loop(ctx context.Context) {
	key := wire.SignKey(b.token)
	for ctx.Err() == nil {
		var frames []wire.Frame
		if b.post("/bridge/v1/poll", nil, &frames) != 200 {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		for _, f := range frames {
			switch f.Type {
			case "revoke", "cancel":
				b.mu.Lock()
				for k, ch := range b.revoked {
					if k == f.GrantID || k == f.CallID {
						close(ch)
						delete(b.revoked, k)
					}
				}
				b.mu.Unlock()
			case "call":
				if wire.Verify(f, key, time.Now()) != nil {
					b.mu.Lock()
					b.badSig++
					b.mu.Unlock()
					continue
				}
				go b.answer(f)
			}
		}
	}
}

func (b *fakeBridge) answer(f wire.Frame) {
	b.mu.Lock()
	b.calls = append(b.calls, f)
	b.mu.Unlock()
	var args struct {
		Argv []string `json:"argv"`
	}
	_ = json.Unmarshal(f.Args, &args)
	zero := 0
	res := wire.Result{OK: true, ExitCode: &zero, Stdout: "ran " + strings.Join(args.Argv, " ")}
	if f.Op == "probe" {
		res = wire.Result{OK: true, Data: json.RawMessage(`{"sandbox":"bubblewrap"}`)}
	}
	if len(args.Argv) > 0 && args.Argv[0] == "sleep" {
		ch := make(chan struct{})
		b.mu.Lock()
		b.revoked[f.GrantID] = ch
		b.mu.Unlock()
		select {
		case <-ch:
			res = wire.Result{OK: false, Error: "grant_revoked", Message: "killed: grant revoked mid-run"}
		case <-time.After(20 * time.Second):
		}
	}
	b.post("/bridge/v1/result", []wire.Frame{{V: 1, Type: "result", CallID: f.CallID, Result: &res}}, nil)
}
