package access

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"agenthub/internal/store"
	"devagotchi/bridge/scope"
)

// Use is the grant a call runs under.
type Use struct {
	GrantID     string         `json:"grant_id"`
	Version     int64          `json:"grant_version"`
	Capability  string         `json:"capability"`
	Level       string         `json:"sandbox_level"`
	Selector    map[string]any `json:"selector"`
	SandboxKind string         `json:"sandbox_kind"`
	Machine     string         `json:"machine_id"`
}

// Authorize is per-call check 1 (03): policy allows the op, then a live grant for this principal, machine,
// capability, selector and level exists (task lease still held, uses left). It consumes one use.
// When a PolicyGate is set it is asked first, and a deny or an unreachable PDP blocks the call.
func (s *Service) Authorize(ctx context.Context, p *store.Principal, machine, action string, c scope.Call) (Use, error) {
	var u Use
	if err := s.policyCheck(ctx, p, machine, c); err != nil {
		return u, err
	}
	_, err := s.St.Write(ctx, p, "", "grant_check", nil, func(w *store.W) (any, error) {
		var disabled, guiPaused bool
		if err := w.Row(`SELECT disabled, gui_paused, sandbox_kind FROM machines WHERE id=$1`, machine).Scan(&disabled, &guiPaused, &u.SandboxKind); err != nil {
			return nil, store.ErrNotFound("machine " + machine)
		}
		if disabled {
			return nil, store.Fail(409, "machine_paused", "machine %s is paused by the operator", machine)
		}
		if guiPaused && strings.HasPrefix(c.Op, "gui.") {
			return nil, store.Fail(409, "gui_stopped_by_operator", "the operator stopped GUI control on this machine")
		}
		if err := w.Can(action); err != nil {
			return nil, err
		}
		capability := scope.CapabilityFor(c.Op)
		rows, err := w.Query(`SELECT g.id::text, g.version, g.capability, g.sandbox_level, g.selector FROM grants g
			WHERE g.principal_id=$1 AND g.machine_id=$2 AND g.ended_at IS NULL AND NOT g.paused AND (g.capability=$3 OR ($3='fs.read' AND g.capability='fs.write'))
			AND (g.expires_at IS NULL OR g.expires_at > now()) AND (g.uses_max IS NULL OR g.uses < g.uses_max)
			AND (g.task_id IS NULL OR g.task_lease_id = (SELECT lease_id FROM tasks t WHERE t.id=g.task_id))
			ORDER BY CASE g.sandbox_level WHEN 'read-only' THEN 0 WHEN 'workspace-write' THEN 1 WHEN 'desktop' THEN 1 ELSE 2 END, g.approved_at
			FOR UPDATE OF g`, w.P.ID, machine, capability)
		if err != nil {
			return nil, err
		}
		found, why := false, ""
		for rows.Next() && !found {
			var raw []byte
			if err := rows.Scan(&u.GrantID, &u.Version, &u.Capability, &u.Level, &raw); err != nil {
				rows.Close()
				return nil, err
			}
			u.Selector = map[string]any{}
			_ = json.Unmarshal(raw, &u.Selector)
			if e := scope.Fits(u.Capability, u.Level, scope.Sel(u.Selector), c); e == nil {
				found = true
			} else {
				why = e.Error()
			}
		}
		rows.Close()
		if !found {
			draft := map[string]any{"machine": machine, "capability": capability, "sandbox_level": firstOr(c.Level, "workspace-write"), "duration": "task",
				"selector": suggest(c), "reason": "(say why, at least 20 characters)"}
			return nil, &store.Error{Status: 403, Code: "grant_required", Message: firstOr(why, "no live grant covers this call") + "; ask with request_access", Current: draft}
		}
		if u.Level != "unsandboxed" && u.SandboxKind == "none" {
			return nil, store.Fail(409, "sandbox_unavailable", "this machine has no working sandbox; only an unsandboxed grant can run here")
		}
		u.Machine = machine
		_, err = w.Exec(`UPDATE grants SET uses=uses+1 WHERE id::text=$1`, u.GrantID)
		return nil, err
	})
	return u, err
}

func suggest(c scope.Call) map[string]any {
	sel := map[string]any{}
	if len(c.Argv) > 0 {
		sel["argv_prefix"] = [][]string{{c.Argv[0]}}
	}
	if len(c.Paths) > 0 {
		sel["paths"] = c.Paths
	}
	if len(c.Hosts) > 0 {
		sel["hosts"] = c.Hosts
	}
	if c.App != "" {
		sel["apps"] = []string{c.App}
	}
	return sel
}

// Fingerprint is the audit form of an argv or path: the first word plus a short hash of the rest.
func Fingerprint(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return parts[0] + "#" + hex.EncodeToString(h[:4])
}

// Used appends grant.used for a finished call (op, fingerprint, exit code; never output bodies).
func (s *Service) Used(ctx context.Context, p *store.Principal, u Use, op, fp string, exit *int, errCode string) {
	_, _ = s.St.Write(ctx, p, "", "grant_used", nil, func(w *store.W) (any, error) {
		return nil, w.Event("grant.used", nil, nil, map[string]any{"grant": u.GrantID, "machine": u.Machine, "op": op, "fp": fp, "exit_code": exit, "error": errCode})
	})
}
