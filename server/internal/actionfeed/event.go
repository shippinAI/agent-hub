// Package actionfeed records every agent action (MCP tool calls, machine commands, file edits, GUI and network
// ops) as typed, redacted events that the web UI renders and that map onto A2UI data-model updates (39).
package actionfeed

import (
	"encoding/json"
	"time"
)

// Kinds. A call produces "call.started" then "call.finished" with the same call_id; MCP tool calls produce one
// "tool.called" event; a denied call produces "call.denied".
const (
	KindStarted  = "call.started"
	KindFinished = "call.finished"
	KindDenied   = "call.denied"
	KindTool     = "tool.called"
)

// Policy is the decision a call ran under: who allowed it and how.
type Policy struct {
	Decision string `json:"decision"` // allow | deny
	GrantID  string `json:"grant_id,omitempty"`
	Level    string `json:"sandbox_level,omitempty"`
	Approver string `json:"approver,omitempty"`
	Code     string `json:"code,omitempty"`   // error code when denied (policy_denied, no_grant, ...)
	Reason   string `json:"reason,omitempty"` // human-readable reason when denied
}

// Event is the wire and storage shape (also the A2UI data model value at /actions/<id>).
type Event struct {
	ID         int64           `json:"id"`
	At         time.Time       `json:"at"`
	Category   string          `json:"category"`
	Kind       string          `json:"kind"`
	CallID     string          `json:"call_id,omitempty"`
	Actor      string          `json:"actor"`
	Machine    string          `json:"machine,omitempty"`
	TaskID     *int64          `json:"task_id,omitempty"`
	Op         string          `json:"op"`
	Tool       string          `json:"tool,omitempty"`
	Argv       []string        `json:"argv,omitempty"`
	Cwd        string          `json:"cwd,omitempty"`
	Paths      []string        `json:"paths,omitempty"`
	Args       json.RawMessage `json:"args,omitempty"`
	Diff       string          `json:"diff,omitempty"`
	Status     string          `json:"status"` // running | ok | error | denied
	ExitCode   *int            `json:"exit_code,omitempty"`
	DurationMS int64           `json:"duration_ms,omitempty"`
	Stdout     string          `json:"stdout,omitempty"`
	Stderr     string          `json:"stderr,omitempty"`
	OutSHA256  string          `json:"out_sha256,omitempty"`
	Truncated  bool            `json:"truncated,omitempty"`
	Error      string          `json:"error,omitempty"`
	Policy     *Policy         `json:"policy,omitempty"`
	Redactions int             `json:"redactions,omitempty"`
}

// CategoryFor maps a machine op (or bridge tool capability) to its audit category.
func CategoryFor(op string) string {
	switch op {
	case "exec", "proc.start", "proc.interactive", "proc.kill", "proc.list", "proc.read", "proc.input":
		return "proc"
	case "fs.read", "fs.write", "fs.list", "fs.stat", "fs.edit", "fs.search":
		return "fs"
	case "net", "net.fetch":
		return "net"
	}
	if len(op) >= 4 && op[:4] == "gui." {
		return "gui"
	}
	return "tool"
}
