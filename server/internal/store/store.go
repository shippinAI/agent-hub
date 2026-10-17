// Package store holds every state change. Each write runs in one transaction that also appends
// the hash-chained event and (optionally) records the idempotency key.
package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store wraps the pool.
type Store struct{ DB *pgxpool.Pool }

// New returns a store on an already-migrated pool.
func New(db *pgxpool.Pool) *Store { return &Store{DB: db} }

// W is the handle a write function gets: the tx, the actor and the request id.
type W struct {
	tx    pgx.Tx
	P     *Principal
	ReqID string
	ctx   context.Context
}

// Replay is a stored response returned for a repeated idempotency key.
type Replay json.RawMessage

// MarshalJSON emits the stored bytes unchanged.
func (r Replay) MarshalJSON() ([]byte, error) { return []byte(r), nil }

// write runs fn in a transaction with idempotency and deadlock retry.
func (s *Store) write(ctx context.Context, p *Principal, reqID, op string, req any, fn func(w *W) (any, error)) (any, error) {
	var out any
	var err error
	for attempt := 0; attempt < 4; attempt++ {
		out, err = s.writeOnce(ctx, p, reqID, op, req, fn)
		var pg *pgconn.PgError
		if errors.As(err, &pg) && (pg.Code == "40P01" || pg.Code == "40001") {
			continue
		}
		break
	}
	return out, err
}

func (s *Store) writeOnce(ctx context.Context, p *Principal, reqID, op string, req any, fn func(w *W) (any, error)) (any, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if reqID != "" {
		if len(reqID) > 200 {
			return nil, errInvalid("request_id longer than 200 chars")
		}
		body, _ := json.Marshal(req)
		sum := sha256.Sum256(append([]byte(op+"\n"), body...))
		h := hex.EncodeToString(sum[:])
		tag, err := tx.Exec(ctx, `INSERT INTO idempotency_keys(principal_id,key,op,request_hash) VALUES($1,$2,$3,$4)
			ON CONFLICT DO NOTHING`, p.ID, reqID, op, h)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() == 0 {
			var prevOp, prevHash string
			var resp []byte
			if err := tx.QueryRow(ctx, `SELECT op, request_hash, response FROM idempotency_keys WHERE principal_id=$1 AND key=$2`,
				p.ID, reqID).Scan(&prevOp, &prevHash, &resp); err != nil {
				return nil, err
			}
			if prevOp != op || prevHash != h || resp == nil {
				return nil, fail(422, "idempotency_mismatch", "request_id %q was already used for a different request", reqID)
			}
			return Replay(resp), nil
		}
	}
	w := &W{tx: tx, P: p, ReqID: reqID, ctx: ctx}
	out, err := fn(w)
	if err != nil {
		return nil, err
	}
	if reqID != "" {
		b, err := json.Marshal(out)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `UPDATE idempotency_keys SET response=$3 WHERE principal_id=$1 AND key=$2`, p.ID, reqID, b); err != nil {
			return nil, err
		}
	}
	return out, tx.Commit(ctx)
}

// ReplayOf reports whether a write result came from the idempotency store.
func ReplayOf(v any) bool { _, ok := v.(Replay); return ok }

func (w *W) exec(sql string, args ...any) (pgconn.CommandTag, error) { return w.tx.Exec(w.ctx, sql, args...) }
func (w *W) row(sql string, args ...any) pgx.Row                    { return w.tx.QueryRow(w.ctx, sql, args...) }
