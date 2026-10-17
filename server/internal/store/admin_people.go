package store

import (
	"context"
	"slices"
	"time"

	"agenthub/internal/policy"
)

// admin is the code-level safety rule for configuration writes: a human principal AND a policy allow.
func (w *W) admin(action string) error {
	if !w.P.IsHuman {
		return fail(403, "humans_only", "configuration changes are limited to human principals")
	}
	return w.can("admin."+action, policy.Resource{})
}

func (w *W) checkKindAndRoles(kind string, roles []string) error {
	st, err := loadSettings(w.ctx, w.tx)
	if err != nil {
		return err
	}
	if kinds := st.Strings("principal_kinds"); len(kinds) > 0 && !slices.Contains(kinds, kind) {
		return errInvalid("kind must be one of %v (edit the principal_kinds setting to add one)", kinds)
	}
	for _, r := range roles {
		var ok bool
		if w.row(`SELECT EXISTS(SELECT 1 FROM roles WHERE id=$1)`, r).Scan(&ok); !ok {
			return errInvalid("unknown role %q", r)
		}
	}
	return nil
}

// PutPrincipal creates (Version 0) or updates a principal. Nobody may change their own roles/human/enabled.
func (s *Store) PutPrincipal(ctx context.Context, p *Principal, reqID string, in Principal) (any, error) {
	return s.write(ctx, p, reqID, "put_principal", in, func(w *W) (any, error) {
		if in.Roles == nil {
			in.Roles = []string{}
		}
		if err := w.checkKindAndRoles(in.Kind, in.Roles); err != nil {
			return nil, err
		}
		if in.Version == 0 {
			if err := w.admin("principals.create"); err != nil {
				return nil, err
			}
			if _, err := w.exec(`INSERT INTO principals(id,display_name,kind,is_human,login,roles,enabled,wip_limit)
				VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, in.ID, in.DisplayName, in.Kind, in.IsHuman, in.Login, in.Roles, in.Enabled, in.WIPLimit); err != nil {
				return nil, errInvalid("principal: %v", err)
			}
		} else {
			if err := w.admin("principals.update"); err != nil {
				return nil, err
			}
			cur, err := scanPrincipal(w.row(`SELECT `+principalCols+` FROM principals WHERE id=$1 FOR UPDATE`, in.ID))
			if err != nil {
				return nil, errNotFound("principal")
			}
			if cur.Version != in.Version {
				return nil, errVersion(cur)
			}
			if in.ID == p.ID && (!slices.Equal(cur.Roles, in.Roles) || cur.IsHuman != in.IsHuman || cur.Enabled != in.Enabled) {
				return nil, fail(403, "self_escalation", "you cannot change your own roles, human flag or enabled state")
			}
			if _, err := w.exec(`UPDATE principals SET display_name=$2,kind=$3,is_human=$4,login=$5,roles=$6,enabled=$7,wip_limit=$8,
				version=version+1 WHERE id=$1`, in.ID, in.DisplayName, in.Kind, in.IsHuman, in.Login, in.Roles, in.Enabled, in.WIPLimit); err != nil {
				return nil, errInvalid("principal: %v", err)
			}
		}
		if err := w.Event("admin.principal.put", nil, nil, map[string]any{"id": in.ID, "kind": in.Kind, "roles": in.Roles,
			"is_human": in.IsHuman, "enabled": in.Enabled, "wip_limit": in.WIPLimit}); err != nil {
			return nil, err
		}
		return scanPrincipal(w.row(`SELECT `+principalCols+` FROM principals WHERE id=$1`, in.ID))
	})
}

// IssuedToken is returned once at creation; Token is the only copy of the secret.
type IssuedToken struct {
	Token string    `json:"token"`
	Info  TokenInfo `json:"info"`
}

// CreateToken issues a token. No idempotency key on purpose: the secret must never be stored for replay.
func (s *Store) CreateToken(ctx context.Context, p *Principal, principal, name string, expiresDays int) (IssuedToken, error) {
	out, err := s.write(ctx, p, "", "create_token", nil, func(w *W) (any, error) {
		if p != System {
			if err := w.admin("tokens.create"); err != nil {
				return nil, err
			}
		}
		tok := NewToken()
		var exp *time.Time
		if expiresDays > 0 {
			exp = ptr(time.Now().Add(time.Duration(expiresDays) * 24 * time.Hour))
		}
		var id string
		if err := w.row(`INSERT INTO tokens(principal_id,name,hash,prefix,expires_at) VALUES($1,$2,$3,$4,$5) RETURNING id::text`,
			principal, name, HashToken(tok), tok[:10], exp).Scan(&id); err != nil {
			return nil, errInvalid("token: %v", err)
		}
		if err := w.Event("token.created", nil, nil, map[string]any{"principal": principal, "token_id": id, "name": name}); err != nil {
			return nil, err
		}
		return IssuedToken{Token: tok, Info: TokenInfo{ID: id, PrincipalID: principal, Name: name, Prefix: tok[:10], ExpiresAt: exp}}, nil
	})
	if err != nil {
		return IssuedToken{}, err
	}
	return out.(IssuedToken), nil
}

// RevokeToken marks a token revoked (rows are kept for audit).
func (s *Store) RevokeToken(ctx context.Context, p *Principal, reqID, id string) (any, error) {
	return s.write(ctx, p, reqID, "revoke_token", id, func(w *W) (any, error) {
		if err := w.admin("tokens.revoke"); err != nil {
			return nil, err
		}
		var owner string
		if err := w.row(`UPDATE tokens SET revoked_at=now() WHERE id=$1::uuid AND revoked_at IS NULL RETURNING principal_id`, id).Scan(&owner); err != nil {
			return nil, errNotFound("live token")
		}
		return map[string]any{"revoked": id}, w.Event("token.revoked", nil, nil, map[string]any{"token_id": id, "principal": owner})
	})
}
