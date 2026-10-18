package machineops

import (
	"context"
	"encoding/json"
	"time"

	"agenthub/internal/store"
	"devagotchi/bridge/scope"
	"devagotchi/bridge/wire"
)

// GUISession is a live GUI lease.
type GUISession struct {
	ID      string `json:"gui_session_id"`
	Machine string `json:"machine_id"`
	Fence   int64  `json:"fence"`
	TTL     int    `json:"ttl_seconds"`
}

// GUIStart takes the machine's single GUI lease under the caller's gui.observe grant and starts the helper session.
func (o *Ops) GUIStart(ctx context.Context, p *store.Principal, machine string, monitors []string, ttl int) (any, error) {
	set, _ := o.St.LoadSettings(ctx)
	if ttl <= 0 || ttl > set.Int("gui_session_ttl_seconds", 600) {
		ttl = set.Int("gui_session_ttl_seconds", 600)
	}
	res, err := o.Call(ctx, p, machine, "gui.observe", "gui.session", map[string]any{"monitors": monitors}, scope.Call{}, 20*time.Second)
	if err != nil {
		return nil, err
	}
	if !res.OK {
		return nil, bridgeErr(res)
	}
	var s GUISession
	_, err = o.St.Write(ctx, p, "", "gui_session_start", nil, func(w *store.W) (any, error) {
		var grants []string
		rows, err := w.Query(`SELECT id::text FROM grants WHERE machine_id=$1 AND principal_id=$2 AND ended_at IS NULL AND capability LIKE 'gui.%'`, machine, w.P.ID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var g string
			_ = rows.Scan(&g)
			grants = append(grants, g)
		}
		rows.Close()
		err = w.Row(`INSERT INTO gui_sessions(machine_id,grant_ids,holder,fence,ttl_seconds,lease_expires_at,state,monitors,display_server)
			VALUES($1,$2::uuid[],$3,nextval('fence_seq'),$4::int,now()+make_interval(secs => $4::int),'active',$5::jsonb,(SELECT display_server FROM machines WHERE id=$1))
			RETURNING id::text, fence`, machine, grants, w.P.ID, ttl, res.Data).Scan(&s.ID, &s.Fence)
		if err != nil {
			return nil, store.Fail(409, "machine_busy", "another GUI session is live on this machine")
		}
		s.Machine, s.TTL = machine, ttl
		return nil, w.Event("gui.session_started", nil, nil, map[string]any{"session": s.ID, "machine": machine, "holder": w.P.ID})
	})
	return map[string]any{"session": s, "display": json.RawMessage(res.Data)}, err
}

// guiSession checks the caller holds the live session with this fence (stale_fence otherwise).
func (o *Ops) guiSession(ctx context.Context, p *store.Principal, id string, fence int64, check bool) (string, error) {
	var machine, holder, state string
	var cur int64
	if err := o.St.DB.QueryRow(ctx, `SELECT machine_id, holder, state, fence FROM gui_sessions WHERE id::text=$1 AND lease_expires_at > now()`, id).Scan(&machine, &holder, &state, &cur); err != nil {
		return "", store.Fail(409, "stale_fence", "GUI session %s is not live", id)
	}
	if holder != p.ID || state != "active" || (check && fence != cur) {
		return "", store.Fail(409, "stale_fence", "you do not hold GUI session %s with that fence", id)
	}
	_, _ = o.St.DB.Exec(ctx, `UPDATE gui_sessions SET lease_expires_at=now()+make_interval(secs => ttl_seconds) WHERE id::text=$1`, id)
	return machine, nil
}

// GUIOp relays screenshot, input, tree, window and app ops to the helper under the matching grant.
func (o *Ops) GUIOp(ctx context.Context, p *store.Principal, session string, fence int64, op string, args map[string]any, app string) (wire.Result, error) {
	machine, err := o.guiSession(ctx, p, session, fence, op == "gui.input")
	if err != nil {
		return wire.Result{}, err
	}
	args["gui_session_id"] = session
	res, err := o.Call(ctx, p, machine, scope.CapabilityFor(op), op, args, scope.Call{App: app}, 30*time.Second)
	if err == nil && op == "gui.screenshot" && res.OK {
		var d struct {
			SHA string `json:"sha256"`
			W   int    `json:"width"`
			H   int    `json:"height"`
			N   int    `json:"bytes"`
		}
		_ = json.Unmarshal(res.Data, &d)
		_, _ = o.St.Write(ctx, p, "", "gui_artifact", nil, func(w *store.W) (any, error) {
			var id string
			if err := w.Row(`INSERT INTO gui_artifacts(gui_session_id,kind,sha256,width,height,bytes) VALUES($1::uuid,'screenshot',$2,$3,$4,$5) RETURNING id::text`,
				session, d.SHA, d.W, d.H, d.N).Scan(&id); err != nil {
				return nil, err
			}
			return nil, w.Event("gui.screenshot", nil, nil, map[string]any{"session": session, "sha256": d.SHA, "width": d.W, "height": d.H, "artifact": id})
		})
	}
	return res, err
}

// GUIEnd ends a session (holder, or a human from the UI).
func (o *Ops) GUIEnd(ctx context.Context, p *store.Principal, session, reason string) (any, error) {
	return o.St.Write(ctx, p, "", "gui_session_end", session, func(w *store.W) (any, error) {
		var machine string
		if err := w.Row(`UPDATE gui_sessions SET state='ended', ended_at=now(), end_reason=$3 WHERE id::text=$1 AND state<>'ended' AND (holder=$2 OR $4)
			RETURNING machine_id`, session, w.P.ID, firstOr(reason, "ended"), w.P.IsHuman).Scan(&machine); err != nil {
			return nil, store.Fail(409, "not_live", "GUI session is not live or not yours")
		}
		o.Hub.Push(machine, wire.Frame{V: wire.Version, Type: "cancel", CallID: "gui:" + session})
		return map[string]any{"ended": session}, w.Event("gui.session_ended", nil, nil, map[string]any{"session": session, "by": w.P.ID, "reason": reason})
	})
}

func firstOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func bridgeErr(r wire.Result) error {
	return store.Fail(422, firstOr(r.Error, "bridge_error"), "%s", firstOr(r.Message, "the bridge refused the call"))
}
