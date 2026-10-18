// Package search is the pure-Go replacement for DC's ripgrep sessions: file-name and content search with
// .gitignore rules, hidden-file and binary skipping, context lines, timeouts, and paged streaming results.
package search

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// rule is one .gitignore line, relative to the directory that holds it.
type rule struct {
	base    string // dir of the .gitignore, slash form, relative to the search root ("" = root)
	pat     string
	neg     bool
	dirOnly bool
	anchor  bool
}

// ignorer holds every rule seen on the way down.
type ignorer struct{ rules []rule }

// load reads dir/.gitignore and dir/.ignore (rg reads both).
func (ig *ignorer) load(root, dir string) {
	for _, name := range []string{".gitignore", ".ignore"} {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(root, dir)
		base := filepath.ToSlash(rel)
		if base == "." {
			base = ""
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			l := strings.TrimRight(sc.Text(), " \t")
			if l == "" || strings.HasPrefix(l, "#") {
				continue
			}
			r := rule{base: base}
			if r.neg = strings.HasPrefix(l, "!"); r.neg {
				l = l[1:]
			}
			if r.dirOnly = strings.HasSuffix(l, "/"); r.dirOnly {
				l = strings.TrimSuffix(l, "/")
			}
			r.anchor = strings.Contains(strings.TrimPrefix(l, "**/"), "/")
			r.pat = strings.TrimPrefix(l, "/")
			ig.rules = append(ig.rules, r)
		}
		f.Close()
	}
}

// ignored applies the rules in order; the last match wins (git semantics).
func (ig *ignorer) ignored(rel string, isDir bool) bool {
	out := false
	for _, r := range ig.rules {
		if r.dirOnly && !isDir {
			continue
		}
		sub := rel
		if r.base != "" {
			if !strings.HasPrefix(rel, r.base+"/") {
				continue
			}
			sub = rel[len(r.base)+1:]
		}
		if matchRule(r, sub) {
			out = !r.neg
		}
	}
	return out
}

func matchRule(r rule, sub string) bool {
	if !r.anchor {
		return globMatch(r.pat, path.Base(sub))
	}
	return globMatch(r.pat, sub)
}

// globMatch is path.Match plus "**" for any number of directories.
func globMatch(pat, name string) bool {
	if !strings.Contains(pat, "**") {
		ok, _ := path.Match(pat, name)
		return ok
	}
	head, tail, _ := strings.Cut(pat, "**")
	head = strings.TrimSuffix(head, "/")
	tail = strings.TrimPrefix(tail, "/")
	parts := strings.Split(name, "/")
	for i := 0; i <= len(parts); i++ {
		pre, post := strings.Join(parts[:i], "/"), strings.Join(parts[i:], "/")
		if (head == "" || globMatch(head, pre)) && (tail == "" || globMatch(tail, post) || globMatch(tail, path.Base(post))) {
			return true
		}
	}
	return false
}
