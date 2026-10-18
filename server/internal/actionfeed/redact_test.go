package actionfeed

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestScrubberMasksSecrets(t *testing.T) {
	s := NewScrubber([]string{"tok_"}, []string{"token", "api_key", "authorization", "password"})
	argv, n := s.Argv([]string{"curl", "--api-key", "plainvalue", "--token=abc123", "-H", "Authorization: Bearer eyJhbGciOi.x.y", "sk-abcdefghijkl"})
	got := strings.Join(argv, " ")
	for _, bad := range []string{"plainvalue", "abc123", "eyJhbGciOi", "abcdefghijkl"} {
		if strings.Contains(got, bad) {
			t.Errorf("leaked %q: %s", bad, got)
		}
	}
	if n < 4 || !strings.Contains(got, "curl") {
		t.Errorf("count %d: %s", n, got)
	}
	raw, n := s.JSON(json.RawMessage(`{"path":"/a","env":{"GITHUB_TOKEN":"ghp_zzzzzzzzzzzz","HOME":"/h"},"password":"hunter2","max_tokens":100,"note":"key tok_123456789"}`))
	out := string(raw)
	for _, bad := range []string{"zzzzzzzzzzzz", "hunter2", "123456789"} {
		if strings.Contains(out, bad) {
			t.Errorf("JSON leaked %q: %s", bad, out)
		}
	}
	if !strings.Contains(out, `"max_tokens":100`) || !strings.Contains(out, `"HOME":"/h"`) || n != 3 {
		t.Errorf("over-redacted or wrong count %d: %s", n, out)
	}
}

func TestDiffFromArgs(t *testing.T) {
	d := DiffFromArgs(map[string]any{"file_path": "a.go", "old_string": "x := 1\n", "new_string": "x := 2\ny := 3\n"})
	if !strings.Contains(d, "--- a/a.go") || !strings.Contains(d, "-x := 1") || !strings.Contains(d, "+y := 3") {
		t.Errorf("edit diff: %s", d)
	}
	if DiffFromArgs(map[string]any{"argv": []any{"ls"}}) != "" {
		t.Error("non-edit args produced a diff")
	}
	if w := DiffFromArgs(map[string]any{"path": "n.txt", "content": "hi"}); !strings.Contains(w, "+hi") {
		t.Errorf("write diff: %s", w)
	}
}

func TestA2UIMessages(t *testing.T) {
	msgs := A2UI([]Event{{ID: 7, Kind: KindFinished, Op: "exec", Status: "ok"}})
	b, _ := json.Marshal(msgs)
	s := string(b)
	for _, want := range []string{`"version":"v0.9"`, `"createSurface"`, `"catalogId":"` + CatalogID, `"componentId":"action_card"`, `"path":"/actions"`, `"id":7`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in %s", want, s)
		}
	}
	if CategoryFor("exec") != "proc" || CategoryFor("fs.write") != "fs" || CategoryFor("gui.click") != "gui" || CategoryFor("x") != "tool" {
		t.Error("CategoryFor")
	}
}
