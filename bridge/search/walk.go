package search

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// maxScanBytes skips huge files in content mode (reported in the session's skipped count).
const maxScanBytes = 64 << 20

// Hooks lets the caller confine and extend the walk.
type Hooks struct {
	Allow   func(path string) bool           // symlink targets must stay inside the policy roots
	Extract func(path string) ([]string, bool) // non-text formats (xlsx) as searchable lines
	Emit    func(Hit) bool                    // false = stop (max results reached)
	Skipped func()
}

// Run walks q.Root and emits hits until done, ctx ends, or Emit says stop.
func Run(ctx context.Context, q Query, h Hooks) error {
	m, err := newMatcher(q)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := func(hit Hit) {
		if !h.Emit(hit) {
			cancel()
		}
	}
	files := make(chan string, 256)
	var wg sync.WaitGroup
	if q.Type != "files" {
		for i := 0; i < runtime.NumCPU(); i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for f := range files {
					if ctx.Err() == nil {
						scanFile(m, q.Root, f, h, stop)
					}
				}
			}()
		}
	}
	w := walker{q: q, m: m, h: h, ctx: ctx, files: files, emit: stop, cancel: cancel}
	w.dir(q.Root)
	close(files)
	wg.Wait()
	return nil
}

type walker struct {
	q      Query
	m      *matcher
	h      Hooks
	ig     ignorer
	ctx    context.Context
	files  chan string
	emit   func(Hit)
	cancel func()
}

func (w *walker) dir(dir string) {
	keep := len(w.ig.rules)
	defer func() { w.ig.rules = w.ig.rules[:keep] }()
	w.ig.load(w.q.Root, dir)
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range ents {
		if w.ctx.Err() != nil {
			return
		}
		name := e.Name()
		full := filepath.Join(dir, name)
		rel, _ := filepath.Rel(w.q.Root, full)
		rel = filepath.ToSlash(rel)
		if name == ".git" || !w.q.IncludeHidden && strings.HasPrefix(name, ".") || w.ig.ignored(rel, e.IsDir()) {
			continue
		}
		if e.Type()&os.ModeSymlink != 0 && (w.h.Allow == nil || !w.h.Allow(full)) {
			continue
		}
		isDir := e.IsDir()
		if e.Type()&os.ModeSymlink != 0 {
			if st, err := os.Stat(full); err == nil && st.IsDir() {
				continue // never follow directory links (no loops, no escapes)
			}
		}
		if isDir {
			w.dir(full)
			continue
		}
		if !w.m.fileOK(rel) {
			continue
		}
		if w.q.Type == "files" {
			if ok, exact := w.m.nameHit(rel); ok {
				w.emit(Hit{File: full})
				if exact && w.q.EarlyStop {
					w.cancel()
				}
			}
			continue
		}
		select {
		case w.files <- full:
		case <-w.ctx.Done():
		}
	}
}

func scanFile(m *matcher, root, f string, h Hooks, emit func(Hit)) {
	var lines []string
	if h.Extract != nil {
		if l, ok := h.Extract(f); ok {
			lines = l
		}
	}
	if lines == nil {
		st, err := os.Stat(f)
		if err != nil || st.Size() > maxScanBytes {
			if h.Skipped != nil {
				h.Skipped()
			}
			return
		}
		b, err := os.ReadFile(f)
		if err != nil || bytes.IndexByte(b[:min(len(b), 8192)], 0) >= 0 {
			return
		}
		lines = strings.Split(string(b), "\n")
	}
	for i, l := range lines {
		if !m.re.MatchString(l) {
			continue
		}
		hit := Hit{File: f, Line: i + 1, Match: strings.TrimRight(l, "\r")}
		if c := m.q.ContextLines; c > 0 {
			hit.Context = strings.Join(lines[max(0, i-c):min(len(lines), i+c+1)], "\n")
		}
		emit(hit)
	}
}
