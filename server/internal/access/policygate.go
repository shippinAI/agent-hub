package access

import (
	"context"

	"agenthub/internal/store"
	"devagotchi/bridge/scope"
)

// PolicyGate is an external policy decision point (gold-policy's PDP) asked before any grant is
// checked. A deny or an unreachable PDP blocks the call; no grant use is consumed.
type PolicyGate interface {
	Decide(ctx context.Context, principal, machine string, c scope.Call) (PolicyVerdict, error)
}

// PolicyVerdict is the PDP's answer.
type PolicyVerdict struct {
	Allow      bool
	Reason     string
	DecisionID string
	Version    int64
}

// policyCheck runs the gate (when configured) and records policy.denied in the hub's own audit chain.
func (s *Service) policyCheck(ctx context.Context, p *store.Principal, machine string, c scope.Call) error {
	if s.Policy == nil {
		return nil
	}
	v, err := s.Policy.Decide(ctx, p.ID, machine, c)
	status, code, msg := 403, "policy_denied", v.Reason
	switch {
	case err != nil:
		status, code, msg = 503, "policy_unavailable", "policy decision point unavailable, failing closed: "+err.Error()
	case v.Allow:
		return nil
	}
	_, _ = s.St.Write(ctx, p, "", "policy_denied", nil, func(w *store.W) (any, error) {
		return nil, w.Event("policy.denied", nil, nil, map[string]any{"machine": machine, "op": c.Op, "fp": Fingerprint(c.Argv),
			"code": code, "decision_id": v.DecisionID, "policy_version": v.Version, "reason": msg})
	})
	return store.Fail(status, code, "%s", msg)
}
