// Package fsops implements the fs_* ops with atomic writes (temp+fsync+rename), exact-match edits and
// sha256 checks. On Linux these run re-executed inside the sandbox (sandbox-init fsop) so the grant's path
// confinement applies to file access too, not just exec.
package fsops

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// Op is one filesystem request.
type Op struct {
	Kind         string `json:"kind"` // read, list, stat, write, edit, move, delete
	Path         string `json:"path"`
	To           string `json:"to,omitempty"`
	Content      string `json:"content,omitempty"`
	ExpectSHA256 string `json:"expect_sha256,omitempty"`
	ExpectAbsent bool   `json:"expect_absent,omitempty"`
	Offset       int64  `json:"offset,omitempty"`
	Limit        int64  `json:"limit,omitempty"`
	Edits        []Edit `json:"edits,omitempty"`
}

// Edit replaces old with new, which must match exactly Count times (default 1).
type Edit struct {
	Old   string `json:"old"`
	New   string `json:"new"`
	Count int    `json:"count,omitempty"`
}

// Result is the op outcome (also the error envelope when Error is set).
type Result struct {
	SHA256  string         `json:"sha256,omitempty"`
	Size    int64          `json:"size,omitempty"`
	Mtime   string         `json:"mtime,omitempty"`
	Content string         `json:"content,omitempty"`
	Entries []DirEntry     `json:"entries,omitempty"`
	Applied int            `json:"applied,omitempty"`
	Error   string         `json:"error,omitempty"`
	Message string         `json:"message,omitempty"`
	Current string         `json:"current,omitempty"`
	Extra   map[string]any `json:"extra,omitempty"`
}

// DirEntry is one fs_list row.
type DirEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

const maxRead = 1 << 20

func sha(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func fail(code, msg string, extra ...map[string]any) Result {
	r := Result{Error: code, Message: msg}
	if len(extra) > 0 {
		if c, ok := extra[0]["current"].(string); ok {
			r.Current = c
		}
	}
	return r
}

// Do runs one op.
func Do(op Op) Result {
	switch op.Kind {
	case "read":
		return doRead(op)
	case "list":
		return doList(op)
	case "stat":
		return doStat(op)
	case "write":
		return doWrite(op)
	case "edit":
		return doEdit(op)
	case "move":
		return doMove(op)
	case "delete":
		return doDelete(op)
	}
	return fail("invalid", "unknown fs op "+op.Kind)
}

// Marshal renders a result as JSON.
func Marshal(r Result) json.RawMessage { b, _ := json.Marshal(r); return b }
