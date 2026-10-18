package search

import (
	"path"
	"regexp"
	"strings"
)

// Query is one start_search request (DC argument names).
type Query struct {
	Root          string
	Pattern       string
	Type          string // files | content
	FilePattern   string // "*.js|*.ts"
	IgnoreCase    bool
	MaxResults    int
	IncludeHidden bool
	ContextLines  int
	Literal       bool
	EarlyStop     bool
}

// Hit is one result row.
type Hit struct {
	File    string `json:"file"`
	Line    int    `json:"line,omitempty"`
	Match   string `json:"match,omitempty"`
	Context string `json:"context,omitempty"`
}

type matcher struct {
	q      Query
	re     *regexp.Regexp
	globs  []string
	isGlob bool
}

func newMatcher(q Query) (*matcher, error) {
	m := &matcher{q: q}
	for _, g := range strings.Split(q.FilePattern, "|") {
		if g = strings.TrimSpace(g); g != "" {
			m.globs = append(m.globs, g)
		}
	}
	if q.Type == "files" {
		m.isGlob = strings.ContainsAny(q.Pattern, "*?[")
		return m, nil
	}
	pat := q.Pattern
	if q.Literal {
		pat = regexp.QuoteMeta(pat)
	}
	if q.IgnoreCase {
		pat = "(?i)" + pat
	}
	re, err := regexp.Compile(pat)
	m.re = re
	return m, err
}

// fileOK applies filePattern to the base name (and the relative path for globs with "/").
func (m *matcher) fileOK(rel string) bool {
	if len(m.globs) == 0 {
		return true
	}
	base := path.Base(rel)
	for _, g := range m.globs {
		if globMatch(g, base) || strings.Contains(g, "/") && globMatch(g, rel) {
			return true
		}
	}
	return false
}

// nameHit reports a file-name match and whether it is exact (for early termination).
func (m *matcher) nameHit(rel string) (bool, bool) {
	base, pat := path.Base(rel), m.q.Pattern
	if m.q.IgnoreCase {
		base, pat = strings.ToLower(base), strings.ToLower(pat)
	}
	if m.isGlob {
		return globMatch(pat, base), false
	}
	return strings.Contains(base, pat), base == pat
}
