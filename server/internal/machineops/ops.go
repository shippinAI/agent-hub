// Package machineops runs agent operations on machines: grant check (access.Authorize), signed call frame,
// routing through bridgehub, presence from completed calls, and grant.used audit.
package machineops

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"

	"agenthub/internal/access"
	"agenthub/internal/actionfeed"
	"agenthub/internal/bridgehub"
	"agenthub/internal/machines"
	"agenthub/internal/store"
	"devagotchi/bridge/scope"
	"devagotchi/bridge/wire"
)

// Ops is the machine operations service.
type Ops struct {
	St       *store.Store
	Access   *access.Service
	Machines *machines.Service
	Hub      *bridgehub.Hub
	Feed     *actionfeed.Recorder // optional: every call lands in the action feed (39)
}

// Call runs op with args under the caller's grant. action is the policy action (exec, fs.read, ...).
func (o *Ops) Call(ctx context.Context, p *store.Principal, machine, action, op string, args any, c scope.Call, timeout time.Duration) (wire.Result, error) {
	if p.Kind == "machine" {
		return wire.Result{}, store.Fail(403, "policy_denied", "machine principals cannot call machine tools")
	}
	if c.Op == "" {
		c.Op = op // the tool op authorizes as the op its capability covers
	}
	feed := o.feedStart(ctx, p, machine, op, args, c)
	set, _ := o.St.LoadSettings(ctx)
	if !o.Hub.Connected(machine, time.Duration(2*set.Int("bridge_poll_seconds", 25)+5)*time.Second) {
		err := store.Fail(503, "machine_offline", "the bridge on %s is not connected", machine)
		feed.denied(ctx, err)
		return wire.Result{}, err
	}
	u, err := o.Access.Authorize(ctx, p, machine, action, c)
	if err != nil {
		feed.denied(ctx, err)
		return wire.Result{}, err
	}
	feed.allowed(ctx, u)
	g := &wire.Grant{Capability: u.Capability, Level: u.Level, Selector: u.Selector, Principal: p.ID}
	res, err := o.send(ctx, machine, u.GrantID, u.Version, 0, op, args, g, timeout)
	feed.finished(ctx, res, err)
	fp := access.Fingerprint(append(append([]string{}, c.Argv...), c.Paths...))
	code := ""
	if err != nil {
		if e, ok := store.AsError(err); ok {
			code = e.Code
		}
	} else if !res.OK {
		code = res.Error
	}
	o.Access.Used(ctx, p, u, op, fp, res.ExitCode, code)
	return res, err
}

// Probe runs the implicitly granted probe op (the machine page's Test button and guide checks).
func (o *Ops) Probe(ctx context.Context, machine string) (wire.Result, error) {
	return o.send(ctx, machine, "probe", 0, 0, "probe", map[string]any{}, &wire.Grant{Capability: "probe", Level: "read-only", Selector: map[string]any{}}, 30*time.Second)
}

// send signs a call frame with the machine key, routes it and records presence from the outcome.
func (o *Ops) send(ctx context.Context, machine, grantID string, ver, fence int64, op string, args any, g *wire.Grant, timeout time.Duration) (wire.Result, error) {
	key, err := o.Machines.SignKeyFor(ctx, machine)
	if err != nil {
		return wire.Result{}, err
	}
	raw, _ := json.Marshal(args)
	f := wire.Frame{V: wire.Version, Type: "call", CallID: newID(), GrantID: grantID, GrantVersion: ver, Fence: fence, Op: op, Args: raw, Grant: g,
		Deadline: time.Now().Add(timeout).UTC()}
	wire.Sign(&f, key)
	res, err := o.Hub.Call(ctx, machine, f)
	switch {
	case err != nil:
		o.Machines.CallDone(ctx, machine, false, err.Error())
	case res.Error == "" || res.ExitCode != nil:
		o.Machines.CallDone(ctx, machine, true, "")
	default:
		o.Machines.CallDone(ctx, machine, false, res.Error+": "+res.Message)
	}
	return res, err
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// GrantsEnded implements access.Notifier: push revoke frames so the bridge kills calls under them.
func (o *Ops) GrantsEnded(ctx context.Context, ended []access.Ended) {
	for _, e := range ended {
		o.Hub.Push(e.MachineID, wire.Frame{V: wire.Version, Type: "revoke", GrantID: e.GrantID})
	}
}
