package store

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"agenthub/internal/policy"
)

// Write runs fn in one transaction with idempotency, deadlock retry and the same-tx event writer.
// Devagotchi packages (machines, access, guides, subs) build on it so every write lands in the hash chain.
func (s *Store) Write(ctx context.Context, p *Principal, reqID, op string, req any, fn func(w *W) (any, error)) (any, error) {
	return s.write(ctx, p, reqID, op, req, fn)
}

// Exec runs a statement in the write transaction.
func (w *W) Exec(sql string, args ...any) (pgconn.CommandTag, error) { return w.exec(sql, args...) }

// Row runs a single-row query in the write transaction.
func (w *W) Row(sql string, args ...any) pgx.Row { return w.row(sql, args...) }

// Query runs a query in the write transaction.
func (w *W) Query(sql string, args ...any) (pgx.Rows, error) { return w.tx.Query(w.ctx, sql, args...) }

// Ctx is the request context of the write.
func (w *W) Ctx() context.Context { return w.ctx }

// Settings reads the settings table inside the transaction.
func (w *W) Settings() (Settings, error) { return loadSettings(w.ctx, w.tx) }

// Can evaluates a data policy for the actor inside the transaction.
func (w *W) Can(action string) error { return w.can(action, policy.Resource{}) }

// CanTask evaluates a data policy against a task-like resource.
func (w *W) CanTask(action, project string, labels []string) error {
	return w.can(action, policy.Resource{Project: project, Labels: labels})
}

// Human is the code rule for grant, preset, unsandboxed-policy, guide publish/edit, secret and enrollment writes:
// the actor must be a human principal AND a data policy must allow the action. Agents can never pass it.
func (w *W) Human(action string) error {
	if !w.P.IsHuman {
		return fail(403, "humans_only", "%s is limited to human principals", action)
	}
	return w.can(action, policy.Resource{})
}

// LoadSettings reads settings outside a write.
func (s *Store) LoadSettings(ctx context.Context) (Settings, error) { return loadSettings(ctx, s.DB) }

// Fail builds a protocol error with a stable code.
func Fail(status int, code, format string, a ...any) *Error { return fail(status, code, format, a...) }

// ErrNotFound is the shared 404.
func ErrNotFound(what string) *Error { return errNotFound(what) }

// ErrInvalid is the shared 400.
func ErrInvalid(format string, a ...any) *Error { return errInvalid(format, a...) }

// ErrVersion is the shared optimistic-concurrency 409.
func ErrVersion(cur any) *Error { return errVersion(cur) }

// Bool reads a boolean setting.
func (s Settings) Bool(key string, fallback bool) bool {
	v := fallback
	if raw, ok := s[key]; ok {
		_ = jsonUnmarshal(raw, &v)
	}
	return v
}

// String reads a string setting.
func (s Settings) String(key, fallback string) string {
	v := fallback
	if raw, ok := s[key]; ok {
		_ = jsonUnmarshal(raw, &v)
	}
	return v
}
