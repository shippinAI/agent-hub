package access

import (
	"context"
	"time"

	"agenthub/internal/store"
)

// PolicyReq enables or disables the unsandboxed policy for the tenant (Machine "") or one machine.
type PolicyReq struct {
	Machine      string     `json:"machine"`
	Allowed      bool       `json:"unsandboxed_allowed"`
	Acknowledged []string   `json:"acknowledged_risk_ids"`
	ExpiresAt    *time.Time `json:"expires_at"`
}

// SetPolicy enables (every unsandboxed_risk_ids entry must be acknowledged) or disables the policy.
// Disabling ends live unsandboxed grants that no other policy row still covers.
func (s *Service) SetPolicy(ctx context.Context, p *store.Principal, reqID string, r PolicyReq) (any, error) {
	var ended []Ended
	out, err := s.St.Write(ctx, p, reqID, "sandbox_policy", r, func(w *store.W) (any, error) {
		act := map[bool]string{true: "enable", false: "disable"}[r.Allowed]
		if err := w.Human("policy.unsandboxed." + act); err != nil {
			return nil, err
		}
		if r.Machine != "" {
			var one int
			if err := w.Row(`SELECT 1 FROM machines WHERE id=$1`, r.Machine).Scan(&one); err != nil {
				return nil, store.ErrNotFound("machine " + r.Machine)
			}
		}
		if r.Allowed {
			set, _ := w.Settings()
			for _, need := range set.Strings("unsandboxed_risk_ids") {
				if !has(r.Acknowledged, need) {
					return nil, &store.Error{Status: 422, Code: "risks_not_acknowledged", Message: "tick every risk first", Current: set.Strings("unsandboxed_risk_ids")}
				}
			}
		}
		if r.Acknowledged == nil {
			r.Acknowledged = []string{}
		}
		if _, err := w.Exec(`INSERT INTO sandbox_policies(machine_id,unsandboxed_allowed,enabled_by,acknowledged_risk_ids,expires_at)
			VALUES(nullif($1,''),$2,$3,$4,$5) ON CONFLICT (tenant_id, coalesce(machine_id,'')) DO UPDATE SET unsandboxed_allowed=$2, enabled_by=$3,
			enabled_at=now(), acknowledged_risk_ids=$4, expires_at=$5, version=sandbox_policies.version+1`,
			r.Machine, r.Allowed, w.P.ID, r.Acknowledged, r.ExpiresAt); err != nil {
			return nil, err
		}
		if !r.Allowed {
			var err error
			ended, err = endGrants(w, "policy_disabled", w.P.ID, `(sandbox_level='unsandboxed' OR full_access) AND NOT `+livePolicyFor+``)
			if err != nil {
				return nil, err
			}
		}
		return map[string]any{"machine": r.Machine, "unsandboxed_allowed": r.Allowed, "ended_grants": len(ended)},
			w.Event("policy.unsandboxed."+act+"d", nil, nil, map[string]any{"machine": r.Machine, "acknowledged": r.Acknowledged, "expires_at": r.ExpiresAt})
	})
	if err == nil {
		s.notify(ctx, ended)
	}
	return out, err
}

// livePolicyFor is livePolicy against the grants row being updated.
const livePolicyFor = `EXISTS (SELECT 1 FROM sandbox_policies sp WHERE sp.unsandboxed_allowed AND (sp.expires_at IS NULL OR sp.expires_at > now())
	AND (sp.machine_id = grants.machine_id OR sp.machine_id IS NULL))`

// Policies lists the policy rows.
func (s *Service) Policies(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.St.DB.Query(ctx, `SELECT to_jsonb(sp) FROM sandbox_policies sp ORDER BY machine_id NULLS FIRST`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var m map[string]any
		if err := rows.Scan(&m); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func has(l []string, x string) bool {
	for _, y := range l {
		if x == y {
			return true
		}
	}
	return false
}
