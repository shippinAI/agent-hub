package access

import (
	"context"

	"agenthub/internal/store"
)

// Revoke ends one grant now; the bridge is told after commit and kills calls under it.
func (s *Service) Revoke(ctx context.Context, p *store.Principal, reqID, id, reason string) (any, error) {
	var ended []Ended
	out, err := s.St.Write(ctx, p, reqID, "grant_revoke", map[string]string{"id": id, "r": reason}, func(w *store.W) (any, error) {
		if err := w.Human("grant.revoke"); err != nil {
			return nil, err
		}
		var err error
		ended, err = endGrants(w, "revoked", w.P.ID, `id::text = $1`, id)
		if err != nil {
			return nil, err
		}
		if len(ended) == 0 {
			return nil, store.Fail(409, "not_live", "grant %s is not live", id)
		}
		_, err = w.Exec(`UPDATE grants SET revoke_reason=nullif($2,'') WHERE id::text=$1`, id, reason)
		return map[string]any{"grant_id": id, "revoked": true}, err
	})
	if err == nil {
		s.notify(ctx, ended)
	}
	return out, err
}

// RevokeAll ends every live grant on a machine (also used when a machine is removed).
func (s *Service) RevokeAll(ctx context.Context, p *store.Principal, reqID, machine, reason string) (any, error) {
	var ended []Ended
	out, err := s.St.Write(ctx, p, reqID, "grant_revoke_all", machine+"/"+reason, func(w *store.W) (any, error) {
		if err := w.Human("grant.revoke"); err != nil {
			return nil, err
		}
		var err error
		ended, err = endGrants(w, firstOr(reason, "revoked"), w.P.ID, `machine_id = $1`, machine)
		if err != nil {
			return nil, err
		}
		return map[string]any{"machine_id": machine, "revoked": len(ended)}, w.Event("machine.revoke_all", nil, nil, map[string]any{"machine": machine, "count": len(ended)})
	})
	if err == nil {
		s.notify(ctx, ended)
	}
	return out, err
}

// Live reports whether a live grant still exists (used by the bridge grant check).
func (s *Service) Live(ctx context.Context, grantID string) (bool, error) {
	var live bool
	err := s.St.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM grants WHERE id::text=$1 AND ended_at IS NULL AND NOT paused
		AND (expires_at IS NULL OR expires_at > now()) AND (uses_max IS NULL OR uses < uses_max))`, grantID).Scan(&live)
	return live, err
}
