package history

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"devagotchi/bridge/redact"
)

func TestRecordRecentStats(t *testing.T) {
	d := t.TempDir()
	l := Open(d, true, 3, 10, redact.New(nil))
	for i := 0; i < 8; i++ {
		l.Record(Call{At: time.Now(), Tool: map[bool]string{true: "a", false: "b"}[i%2 == 0], OK: i != 3, Args: json.RawMessage(`{"k":"sk-abcdefghijkl"}`), Output: strings.Repeat("x", 50)})
	}
	r := l.Recent(10, "", time.Time{})
	if len(r) == 0 || strings.Contains(string(r[0].Args), "abcdefghijkl") || len(r[0].Output) > 40 {
		t.Fatalf("%+v", r)
	}
	l2 := Open(d, true, 3, 10, redact.New(nil))
	if got := l2.Recent(10, "a", time.Time{}); len(got) == 0 || len(got) > 3 {
		t.Fatalf("reload %d", len(got))
	}
	s := l2.Summary()
	if s.Calls != 8 || s.Failures != 1 || s.Top[0] == "" {
		t.Fatalf("%+v", s)
	}
	var nilLog *Log
	nilLog.Record(Call{})
}
