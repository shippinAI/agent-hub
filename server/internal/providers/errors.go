package providers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// FieldError is one validation failure at a JSON path (e.g. "adapter.config.argv").
type FieldError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// Errs collects field errors in order.
type Errs []FieldError

func (e *Errs) add(path, format string, a ...any) {
	*e = append(*e, FieldError{Path: path, Message: fmt.Sprintf(format, a...)})
}

// Error joins the messages (for logs and seed failures).
func (e Errs) Error() string {
	parts := make([]string, len(e))
	for i, f := range e {
		parts[i] = f.Path + ": " + f.Message
	}
	return strings.Join(parts, "; ")
}

// decodeStrict unmarshals raw into v and refuses unknown fields, so a typo or a removed field
// (terms_status) is an error instead of being silently dropped.
func decodeStrict(raw []byte, v any, path string, errs *Errs) bool {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		errs.add(errPath(path, err), "%s", jsonMsg(err))
		return false
	}
	return true
}

// errPath points a decode error at the field it is about (type errors and unknown fields).
func errPath(path string, err error) string {
	var te *json.UnmarshalTypeError
	if errors.As(err, &te) && te.Field != "" {
		return join(path, te.Field)
	}
	if f, ok := strings.CutPrefix(err.Error(), `json: unknown field "`); ok {
		return join(path, strings.TrimSuffix(f, `"`))
	}
	return path
}

func jsonMsg(err error) string {
	var te *json.UnmarshalTypeError
	if errors.As(err, &te) {
		return fmt.Sprintf("field %s must be %s, not %s", te.Field, te.Type, te.Value)
	}
	return strings.TrimPrefix(err.Error(), "json: ")
}

func join(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}
