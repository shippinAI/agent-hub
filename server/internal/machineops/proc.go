package machineops

import (
	"context"
	"time"

	"agenthub/internal/access"
	"agenthub/internal/store"
	"devagotchi/bridge/scope"
	"devagotchi/bridge/wire"
)

// ProcStart starts a long-running process under a proc.interactive grant and returns its lease.
func (o *Ops) ProcStart(ctx context.Context, p *store.Principal, machine string, argv []string, cwd string, ttl int, taskID *int64) (any, error) {
	if ttl <= 0 || ttl > 3600 {
		ttl = 300
	}
	var lease string
	var fence int64
	_, err := o.St.Write(ctx, p, "", "proc_lease", nil, func(w *store.W) (any, error) {
		return nil, w.Row(`SELECT gen_random_uuid()::text, nextval('fence_seq')`).Scan(&lease, &fence)
	})
	if err != nil {
		return nil, err
	}
	c := scope.Call{Argv: argv}
	if cwd != "" {
		c.Paths = []string{cwd}
	}
	res, err := o.Call(ctx, p, machine, "proc.start", "proc.start", map[string]any{"argv": argv, "cwd": cwd, "lease_id": lease, "fence": fence, "ttl_s": ttl}, c, 30*time.Second)
	if err != nil {
		return nil, err
	}
	if !res.OK {
		return nil, bridgeErr(res)
	}
	_, err = o.St.Write(ctx, p, "", "proc_started", nil, func(w *store.W) (any, error) {
		_, err := w.Exec(`INSERT INTO proc_leases(id,machine_id,grant_id,holder,task_id,argv_fp,fence,ttl_seconds,lease_expires_at)
			SELECT $1::uuid,$2,g.id,$3,$4,$5,$6,$7,now()+make_interval(secs => $7) FROM grants g WHERE g.machine_id=$2 AND g.principal_id=$3
			AND g.capability='proc.interactive' AND g.ended_at IS NULL ORDER BY g.approved_at DESC LIMIT 1`, lease, machine, w.P.ID, taskID, access.Fingerprint(argv), fence, ttl)
		if err != nil {
			return nil, err
		}
		return nil, w.Event("proc.started", taskID, nil, map[string]any{"lease": lease, "machine": machine, "fp": access.Fingerprint(argv)})
	})
	return map[string]any{"lease_id": lease, "fence": fence, "ttl_seconds": ttl, "data": res.Data}, err
}

// ProcOp is proc.read, proc.write, proc.heartbeat or proc.kill on the caller's lease (fence checked).
func (o *Ops) ProcOp(ctx context.Context, p *store.Principal, lease string, fence int64, op string, args map[string]any) (wire.Result, error) {
	var machine, holder, state string
	var cur int64
	if err := o.St.DB.QueryRow(ctx, `SELECT machine_id, holder, state, fence FROM proc_leases WHERE id::text=$1`, lease).Scan(&machine, &holder, &state, &cur); err != nil {
		return wire.Result{}, store.ErrNotFound("proc lease " + lease)
	}
	if (holder != p.ID && !(p.IsHuman && op == "proc.kill")) || cur != fence && !p.IsHuman {
		return wire.Result{}, store.Fail(409, "stale_fence", "you do not hold lease %s with that fence", lease)
	}
	if state != "running" {
		return wire.Result{}, store.Fail(409, "lease_ended", "process lease is %s", state)
	}
	args["lease_id"], args["fence"] = lease, cur
	var res wire.Result
	var err error
	if p.IsHuman && op == "proc.kill" {
		res, err = o.send(ctx, machine, "operator", 0, cur, op, args, &wire.Grant{Capability: "proc.kill", Level: "read-only", Selector: map[string]any{}, Principal: p.ID}, 20*time.Second)
	} else {
		res, err = o.Call(ctx, p, machine, "proc.*", op, args, scope.Call{}, 30*time.Second)
	}
	if err != nil {
		return res, err
	}
	_, _ = o.St.Write(ctx, p, "", "proc_update", nil, func(w *store.W) (any, error) {
		switch {
		case op == "proc.heartbeat":
			_, err = w.Exec(`UPDATE proc_leases SET heartbeat_at=now(), lease_expires_at=now()+make_interval(secs => ttl_seconds) WHERE id::text=$1`, lease)
		case op == "proc.kill" || res.ExitCode != nil:
			st := map[bool]string{true: "killed", false: "exited"}[op == "proc.kill"]
			_, err = w.Exec(`UPDATE proc_leases SET state=$2, exit_code=$3, ended_at=now() WHERE id::text=$1`, lease, st, res.ExitCode)
		}
		return nil, err
	})
	return res, nil
}
