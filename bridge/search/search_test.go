package search

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func tree(t *testing.T) string {
	d := t.TempDir()
	w := func(p, s string) {
		_ = os.MkdirAll(filepath.Dir(filepath.Join(d, p)), 0o755)
		_ = os.WriteFile(filepath.Join(d, p), []byte(s), 0o644)
	}
	w(".gitignore", "build/\n*.log\n!keep.log\n")
	w("src/main.go", "package main\n// TODO fix\nfunc main() {}\n")
	w("src/util.ts", "const toast = 1; toast.error('test')\n")
	w("build/out.go", "// TODO ignored\n")
	w("app.log", "TODO in log\n")
	w("keep.log", "TODO kept\n")
	w(".hidden/x.go", "// TODO hidden\n")
	w("bin.dat", "TODO\x00binary")
	return d
}

func run(t *testing.T, q Query) []Hit {
	m := NewManager()
	s := m.Start(q, 5*time.Second, 1000, Hooks{})
	s.Wait(5 * time.Second)
	hits, st := s.Page(0, 100)
	if !st.Complete {
		t.Fatal("not complete")
	}
	return hits
}

func TestContentSearch(t *testing.T) {
	d := tree(t)
	hits := run(t, Query{Root: d, Pattern: "todo", Type: "content", IgnoreCase: true})
	got := map[string]bool{}
	for _, h := range hits {
		rel, _ := filepath.Rel(d, h.File)
		got[rel] = true
	}
	if !got["src/main.go"] || !got["keep.log"] || got["app.log"] || got["build/out.go"] || got[".hidden/x.go"] || got["bin.dat"] {
		t.Fatalf("%v", got)
	}
	lit := run(t, Query{Root: d, Pattern: "toast.error('test')", Type: "content", Literal: true, ContextLines: 1})
	if len(lit) != 1 || lit[0].Line != 1 || !strings.Contains(lit[0].Context, "toast") {
		t.Fatalf("%+v", lit)
	}
}

func TestFileSearch(t *testing.T) {
	d := tree(t)
	if hits := run(t, Query{Root: d, Pattern: "*.go", Type: "files", IgnoreCase: true}); len(hits) != 1 {
		t.Fatalf("%+v", hits)
	}
	if hits := run(t, Query{Root: d, Pattern: "main.go", Type: "files", EarlyStop: true}); len(hits) != 1 {
		t.Fatalf("%+v", hits)
	}
	if hits := run(t, Query{Root: d, Pattern: "x", Type: "files", IncludeHidden: true, FilePattern: "*.go"}); len(hits) != 1 {
		t.Fatalf("hidden %+v", hits)
	}
}
