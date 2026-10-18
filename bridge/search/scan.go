package search

import (
	"bytes"
	"os"
	"strings"
)

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
