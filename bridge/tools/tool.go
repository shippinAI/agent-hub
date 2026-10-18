// Package tools defines every Desktop Commander-equivalent tool once, transport-free: the standalone MCP
// server (mcpserve) and the hub-connected runner both call these. Each tool names the capability it needs;
// the policy.Gate decides, so the same code is grant-checked in both modes.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/google/jsonschema-go/jsonschema"

	"devagotchi/bridge/history"
	"devagotchi/bridge/policy"
	"devagotchi/bridge/procs"
	"devagotchi/bridge/redact"
	"devagotchi/bridge/search"
)

// Image is image content returned to the caller (never stored).
type Image struct {
	Mime string
	Data []byte
}

// Out is a tool result. Code set = error (MCP isError) with that error code.
type Out struct {
	Text   string
	Images []Image
	Data   any
	Code   string
}

// Env is what a tool may use. Gate is the only authority on access.
type Env struct {
	Gate       *policy.Gate
	Procs      *procs.Manager
	Searches   *search.Manager
	History    *history.Log
	Redact     redact.Redactor
	Self       string // this binary (in-sandbox init)
	ConfigPath string // policy file shown by get_config
	Mode       string // standalone | hub
	Client     string
	Started    time.Time
	Log        func(string)
	stop       chan struct{}
}

// Tool is one tool definition.
type Tool struct {
	Name        string
	Title       string
	Desc        string
	Cap         string // capability checked ("" = none)
	ReadOnly    bool
	Destructive bool
	OpenWorld   bool
	Schema      *jsonschema.Schema
	Run         func(context.Context, *Env, json.RawMessage) Out
}

// def binds a typed handler: the input schema comes from In's fields and tags.
func def[In any](t Tool, fn func(context.Context, *Env, In) Out) Tool {
	s, err := jsonschema.For[In](&jsonschema.ForOptions{})
	if err != nil {
		panic(fmt.Sprintf("schema for %s: %v", t.Name, err))
	}
	s.AdditionalProperties = nil // accept DC-era extra fields (origin, options) instead of failing
	t.Schema = s
	t.Run = func(ctx context.Context, e *Env, raw json.RawMessage) Out {
		var in In
		if len(raw) > 0 && string(raw) != "null" {
			if err := json.Unmarshal(raw, &in); err != nil {
				return fail("invalid", "arguments do not match the schema: %v", err)
			}
		}
		return fn(ctx, e, in)
	}
	return t
}

func fail(code, format string, a ...any) Out {
	return Out{Code: code, Text: "Error (" + code + "): " + fmt.Sprintf(format, a...)}
}

// errOut maps Go errors onto the shared error codes.
func errOut(err error) Out {
	var d *policy.Denied
	switch {
	case errors.As(err, &d):
		return fail(d.Code, "%s", d.Msg)
	case errors.Is(err, fs.ErrNotExist):
		return fail("not_found", "%v", err)
	case errors.Is(err, fs.ErrPermission):
		return fail("os_denied", "%v (the OS refused; the sandbox or file permissions block it)", err)
	case errors.Is(err, context.DeadlineExceeded):
		return fail("timeout", "%v", err)
	}
	return fail("io_error", "%v", err)
}

func ok(text string, data any) Out { return Out{Text: text, Data: data} }

// timeout applies the policy default and cap, and says so when it clamps (DC #447 capped silently).
func timeout(e *Env, ms int) (time.Duration, string) {
	l := e.Gate.Limits()
	if ms <= 0 {
		return time.Duration(l.DefaultTimeoutMS) * time.Millisecond, ""
	}
	if ms > l.MaxTimeoutMS {
		return time.Duration(l.MaxTimeoutMS) * time.Millisecond, fmt.Sprintf("timeout_ms %d is above this policy's max_timeout_ms; waited %d ms instead. ", ms, l.MaxTimeoutMS)
	}
	return time.Duration(ms) * time.Millisecond, ""
}

func truthy(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}
