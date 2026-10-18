// Package fsops implements the fs_* ops with atomic writes (temp+fsync+rename), exact-match edits and
// sha256 checks. On Linux these run re-executed inside the sandbox (sandbox-init fsop) so the grant's path
// confinement applies to file access too, not just exec.
package fsops

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
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

func doRead(op Op) Result {
	f, err := os.Open(op.Path)
	if err != nil {
		return fail("not_found", err.Error())
	}
	defer f.Close()
	st, _ := f.Stat()
	if op.Offset > 0 {
		_, _ = f.Seek(op.Offset, io.SeekStart)
	}
	lim := int64(maxRead)
	if op.Limit > 0 && op.Limit < lim {
		lim = op.Limit
	}
	b, err := io.ReadAll(io.LimitReader(f, lim))
	if err != nil {
		return fail("io_error", err.Error())
	}
	return Result{Content: string(b), Size: st.Size(), SHA256: sha(b), Mtime: st.ModTime().UTC().Format(time.RFC3339)}
}

func doList(op Op) Result {
	ents, err := os.ReadDir(op.Path)
	if err != nil {
		return fail("not_found", err.Error())
	}
	out := Result{Entries: []DirEntry{}}
	for _, e := range ents {
		info, _ := e.Info()
		var sz int64
		if info != nil {
			sz = info.Size()
		}
		out.Entries = append(out.Entries, DirEntry{Name: e.Name(), IsDir: e.IsDir(), Size: sz})
	}
	return out
}

func doStat(op Op) Result {
	st, err := os.Stat(op.Path)
	if err != nil {
		return fail("not_found", err.Error())
	}
	return Result{Size: st.Size(), Mtime: st.ModTime().UTC().Format(time.RFC3339), Extra: map[string]any{"is_dir": st.IsDir(), "mode": st.Mode().String()}}
}

func current(path string) (string, []byte, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", nil, false
	}
	return sha(b), b, true
}

func doWrite(op Op) Result {
	cur, _, exists := current(op.Path)
	if op.ExpectAbsent && exists {
		return fail("hash_mismatch", "file already exists", map[string]any{"current": cur})
	}
	if !op.ExpectAbsent && exists && op.ExpectSHA256 != "" && cur != op.ExpectSHA256 {
		return fail("hash_mismatch", "file changed since you read it", map[string]any{"current": cur})
	}
	if err := atomicWrite(op.Path, []byte(op.Content)); err != nil {
		return fail("io_error", err.Error())
	}
	return Result{SHA256: sha([]byte(op.Content)), Size: int64(len(op.Content))}
}

func doEdit(op Op) Result {
	cur, b, ok := current(op.Path)
	if !ok {
		return fail("not_found", "file not found")
	}
	if op.ExpectSHA256 != "" && cur != op.ExpectSHA256 {
		return fail("hash_mismatch", "file changed since you read it", map[string]any{"current": cur})
	}
	s := string(b)
	n := 0
	for _, e := range op.Edits {
		want := e.Count
		if want == 0 {
			want = 1
		}
		got := strings.Count(s, e.Old)
		if got == 0 {
			return fail("edit_no_match", fmt.Sprintf("edit %q matched nothing", trunc(e.Old)))
		}
		if got != want {
			return fail("edit_ambiguous", fmt.Sprintf("edit %q matched %d times, expected %d", trunc(e.Old), got, want))
		}
		s = strings.ReplaceAll(s, e.Old, e.New)
		n++
	}
	if err := atomicWrite(op.Path, []byte(s)); err != nil {
		return fail("io_error", err.Error())
	}
	return Result{SHA256: sha([]byte(s)), Applied: n, Size: int64(len(s))}
}

func doMove(op Op) Result {
	cur, _, ok := current(op.Path)
	if !ok {
		return fail("not_found", "source not found")
	}
	if op.ExpectSHA256 != "" && cur != op.ExpectSHA256 {
		return fail("hash_mismatch", "source changed", map[string]any{"current": cur})
	}
	if err := os.Rename(op.Path, op.To); err != nil {
		return fail("io_error", err.Error())
	}
	return Result{Extra: map[string]any{"moved": true}}
}

func doDelete(op Op) Result {
	cur, _, ok := current(op.Path)
	if !ok {
		return fail("not_found", "file not found")
	}
	if op.ExpectSHA256 == "" {
		return fail("invalid", "delete needs expect_sha256")
	}
	if cur != op.ExpectSHA256 {
		return fail("hash_mismatch", "file changed", map[string]any{"current": cur})
	}
	if err := os.Remove(op.Path); err != nil {
		return fail("io_error", err.Error())
	}
	return Result{Extra: map[string]any{"deleted": true}}
}

func atomicWrite(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".devagotchi-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func trunc(s string) string {
	if len(s) > 40 {
		return s[:40] + "…"
	}
	return s
}

// Marshal renders a result as JSON.
func Marshal(r Result) json.RawMessage { b, _ := json.Marshal(r); return b }
