package access

import (
	"context"

	"agenthub/internal/store"
	"devagotchi/bridge/scope"
)

// PolicyGate is a policy decision point asked before any grant is checked: the hub's own change-policy
// module and, optionally, an external PDP. A deny or an error blocks the call; no grant use is consumed.
type PolicyGate interface {
	Decide(ctx context.Context, p *store.Principal, machine string, c scope.Call) (PolicyVerdict, error)
}

// Gates asks each gate in order; the first deny or error wins (strictest wins).
type Gates []PolicyGate

// Decide implements PolicyGate.
func (g Gates) Decide(ctx context.Context, p *store.Principal, machine string, c scope.Call) (PolicyVerdict, error) {
	v := PolicyVerdict{Allow: true}
	for _, one := range g {
		var err error
		if v, err = one.Decide(ctx, p, machine, c); err != nil || !v.Allow {
			return v, err
		}
	}
	return v, nil
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
	v, err := s.Policy.Decide(ctx, p, machine, c)
	status, code, msg := 403, "policy_denied", v.Reason
	switch {
	case err != nil:
		status, code, msg = 503, "policy_unavailable", "policy decision point unavailable, failing closed: "+err.Error()
		if e, ok := store.AsError(err); ok && e.Code == "policy_unavailable" {
			msg = e.Message
		}
	case v.Allow:
		return nil
	}
	_, _ = s.St.Write(ctx, p, "", "policy_denied", nil, func(w *store.W) (any, error) {
		return nil, w.Event("policy.denied", nil, nil, map[string]any{"machine": machine, "op": c.Op, "fp": Fingerprint(c.Argv),
			"code": code, "decision_id": v.DecisionID, "policy_version": v.Version, "reason": msg})
	})
	return store.Fail(status, code, "%s", msg)
}
