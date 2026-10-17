package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"

	"github.com/jackc/pgx/v5"
)

const principalCols = `id,display_name,kind,is_human,login,roles,enabled,wip_limit,last_seen_at,version,created_at`

func scanPrincipal(r pgx.Row) (Principal, error) {
	var p Principal
	err := r.Scan(&p.ID, &p.DisplayName, &p.Kind, &p.IsHuman, &p.Login, &p.Roles, &p.Enabled, &p.WIPLimit, &p.LastSeenAt, &p.Version, &p.CreatedAt)
	return p, err
}

// Principals lists everyone.
func (s *Store) Principals(ctx context.Context) ([]Principal, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+principalCols+` FROM principals ORDER BY is_human DESC, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Principal{}
	for rows.Next() {
		p, err := scanPrincipal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// HashToken is how tokens are stored and looked up; plaintext never touches the DB.
func HashToken(tok string) []byte { s := sha256.Sum256([]byte(tok)); return s[:] }

// NewToken returns a fresh "ahk_..." secret.
func NewToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return "ahk_" + base64.RawURLEncoding.EncodeToString(b)
}

// PrincipalByToken resolves an enabled principal from a live token and marks it seen.
func (s *Store) PrincipalByToken(ctx context.Context, tok string) (*Principal, error) {
	p, err := scanPrincipal(s.DB.QueryRow(ctx, `SELECT `+principalCols+` FROM principals WHERE enabled AND id=(
		SELECT principal_id FROM tokens WHERE hash=$1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now()))`, HashToken(tok)))
	if err != nil {
		return nil, errNotFound("token")
	}
	s.touch(ctx, &p, HashToken(tok))
	return &p, nil
}

// PrincipalByLogin maps an external human login (tailnet, proxy header) to a principal.
func (s *Store) PrincipalByLogin(ctx context.Context, login string) (*Principal, error) {
	p, err := scanPrincipal(s.DB.QueryRow(ctx, `SELECT `+principalCols+` FROM principals WHERE enabled AND login=$1`, login))
	if err != nil {
		return nil, errNotFound("principal for login")
	}
	s.touch(ctx, &p, nil)
	return &p, nil
}

func (s *Store) touch(ctx context.Context, p *Principal, hash []byte) {
	if p.LastSeenAt != nil && time.Since(*p.LastSeenAt) < 15*time.Second {
		return
	}
	_, _ = s.DB.Exec(ctx, `UPDATE principals SET last_seen_at=now() WHERE id=$1`, p.ID)
	if hash != nil {
		_, _ = s.DB.Exec(ctx, `UPDATE tokens SET last_used_at=now() WHERE hash=$1`, hash)
	}
}

// TokenInfo is a token's metadata (never the secret).
type TokenInfo struct {
	ID          string     `json:"id"`
	PrincipalID string     `json:"principal_id"`
	Name        string     `json:"name"`
	Prefix      string     `json:"prefix"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	ExpiresAt   *time.Time `json:"expires_at"`
	RevokedAt   *time.Time `json:"revoked_at"`
}

// Tokens lists a principal's tokens.
func (s *Store) Tokens(ctx context.Context, principal string) ([]TokenInfo, error) {
	rows, err := s.DB.Query(ctx, `SELECT id::text,principal_id,name,prefix,created_at,last_used_at,expires_at,revoked_at
		FROM tokens WHERE principal_id=$1 ORDER BY created_at DESC`, principal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TokenInfo{}
	for rows.Next() {
		var t TokenInfo
		if err := rows.Scan(&t.ID, &t.PrincipalID, &t.Name, &t.Prefix, &t.CreatedAt, &t.LastUsedAt, &t.ExpiresAt, &t.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
