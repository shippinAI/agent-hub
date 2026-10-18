package machineops

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"agenthub/internal/access"
	"agenthub/internal/actionfeed"
	"agenthub/internal/store"
	"devagotchi/bridge/scope"
	"devagotchi/bridge/wire"
)

// feedCall describes one machine call for the action feed (39). Nil Feed = feed off.
type feedCall struct {
	o       *Ops
	p       *store.Principal
	machine string
	op      string
	callID  string
	base    actionfeed.Event
	start   time.Time
}

func (o *Ops) feedStart(ctx context.Context, p *store.Principal, machine, op string, args any, c scope.Call) *feedCall {
	if o.Feed == nil {
		return nil
	}
	raw, _ := json.Marshal(args)
	f := &feedCall{o: o, p: p, machine: machine, op: op, callID: newID(), start: time.Now()}
	f.base = actionfeed.Event{CallID: f.callID, Actor: p.ID, Machine: machine, Op: c.Op, Argv: c.Argv, Paths: c.Paths, Args: raw}
	if op == "tool" {
		var t struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		_ = json.Unmarshal(raw, &t)
		f.base.Tool, f.base.Diff = t.Name, actionfeed.DiffFromArgs(t.Arguments)
		if cwd, ok := t.Arguments["cwd"].(string); ok {
			f.base.Cwd = cwd
		}
	} else {
		var a map[string]any
		_ = json.Unmarshal(raw, &a)
		if cwd, ok := a["cwd"].(string); ok {
			f.base.Cwd = cwd
		}
		f.base.Diff = actionfeed.DiffFromArgs(a)
	}
	f.base.Category = actionfeed.CategoryFor(c.Op)
	return f
}

func (f *feedCall) record(ctx context.Context, e actionfeed.Event) {
	if _, err := f.o.Feed.Record(context.WithoutCancel(ctx), e); err != nil {
		log.Printf("actionfeed: %v", err)
	}
}

// denied records a call refused before it reached the bridge (policy, grant, offline machine).
func (f *feedCall) denied(ctx context.Context, err error) {
	if f == nil {
		return
	}
	e := f.base
	e.Kind, e.Status = actionfeed.KindDenied, "denied"
	pol := &actionfeed.Policy{Decision: "deny", Reason: err.Error()}
	if se, ok := store.AsError(err); ok {
		pol.Code, pol.Reason = se.Code, se.Message
	}
	e.Policy, e.Error = pol, pol.Reason
	f.record(ctx, e)
}

// allowed records the start of an authorized call with the grant, level and the human who approved it.
func (f *feedCall) allowed(ctx context.Context, u access.Use) {
	if f == nil {
		return
	}
	var approver string
	_ = f.o.St.DB.QueryRow(ctx, `SELECT approved_by FROM grants WHERE id::text=$1`, u.GrantID).Scan(&approver)
	f.base.Policy = &actionfeed.Policy{Decision: "allow", GrantID: u.GrantID, Level: u.Level, Approver: approver}
	e := f.base
	e.Kind, e.Status = actionfeed.KindStarted, "running"
	f.record(ctx, e)
}

// finished records the outcome: exit code, output streams (redacted, capped), duration and error.
func (f *feedCall) finished(ctx context.Context, res wire.Result, err error) {
	if f == nil {
		return
	}
	e := f.base
	e.Kind, e.ExitCode, e.Stdout, e.Stderr = actionfeed.KindFinished, res.ExitCode, res.Stdout, res.Stderr
	e.OutSHA256, e.Truncated, e.DurationMS = res.OutSHA256, res.Truncated, res.DurationMS
	if e.DurationMS == 0 {
		e.DurationMS = time.Since(f.start).Milliseconds()
	}
	switch {
	case err != nil:
		e.Status, e.Error = "error", err.Error()
	case !res.OK || (res.ExitCode != nil && *res.ExitCode != 0):
		e.Status, e.Error = "error", joinErr(res.Error, res.Message)
	default:
		e.Status = "ok"
	}
	f.record(ctx, e)
}

func joinErr(code, msg string) string {
	if code != "" && msg != "" {
		return code + ": " + msg
	}
	return code + msg
}
