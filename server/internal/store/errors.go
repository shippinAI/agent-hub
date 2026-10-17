package store

import (
	"errors"
	"fmt"
)

// Error is a protocol error with a stable code shared by REST and MCP.
type Error struct {
	Code    string `json:"code"`
	Status  int    `json:"-"`
	Message string `json:"message"`
	Current any    `json:"current,omitempty"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func fail(status int, code, format string, a ...any) *Error {
	return &Error{Code: code, Status: status, Message: fmt.Sprintf(format, a...)}
}

// Codes used across interfaces (see docs/spec.md).
func errNotFound(what string) *Error { return fail(404, "not_found", "%s not found", what) }
func errDenied(action, rule string) *Error {
	return fail(403, "policy_denied", "%s denied by %s", action, rule)
}
func errInvalid(format string, a ...any) *Error { return fail(400, "invalid", format, a...) }
func errVersion(cur any) *Error {
	e := fail(409, "version_conflict", "version does not match; re-read and retry")
	e.Current = cur
	return e
}

// AsError unwraps a store error, if any.
func AsError(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}
