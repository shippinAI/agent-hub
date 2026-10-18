package fsops

import (
	"fmt"
	"os"
	"strings"
)

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
