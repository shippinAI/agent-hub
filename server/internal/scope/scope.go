// Package scope holds the pure rules for grant scopes: is one scope within another (narrowing), does a call
// fit a grant, and which risk rows a selector triggers. Used by the hub and by the bridge's own recheck.
package scope

import (
	"fmt"
	"path"
	"reflect"
	"strconv"
	"strings"
)

// Sel is a decoded selector.
type Sel map[string]any

// Strs reads a string list; the second result is false when the key is absent.
func (s Sel) Strs(k string) ([]string, bool) {
	v, ok := s[k]
	if !ok || v == nil {
		return nil, false
	}
	l, _ := v.([]any)
	out := []string{}
	for _, x := range l {
		out = append(out, fmt.Sprint(x))
	}
	if ss, ok := v.([]string); ok {
		out = ss
	}
	return out, true
}

// Prefixes reads argv_prefix as a list of string lists.
func (s Sel) Prefixes() [][]string {
	l, _ := s["argv_prefix"].([]any)
	out := [][]string{}
	for _, p := range l {
		var one []string
		pl, _ := p.([]any)
		for _, x := range pl {
			one = append(one, fmt.Sprint(x))
		}
		out = append(out, one)
	}
	return out
}

// Any reports whether key holds the literal "any".
func (s Sel) Any(k string) bool { return fmt.Sprint(s[k]) == "any" }

// Num reads a number (ok=false when absent).
func (s Sel) Num(k string) (float64, bool) {
	switch v := s[k].(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	}
	return 0, false
}

// Within reports whether p is a or below it (cleaned, "/"-separated; "~" stays literal for the bridge).
func Within(p, a string) bool {
	p, a = path.Clean(strings.ReplaceAll(p, `\`, "/")), path.Clean(strings.ReplaceAll(a, `\`, "/"))
	return p == a || a == "/" || strings.HasPrefix(p, strings.TrimSuffix(a, "/")+"/")
}

// HasPrefix reports whether argv starts with prefix, element by element (never as one string).
func HasPrefix(argv, prefix []string) bool {
	if len(prefix) > len(argv) {
		return false
	}
	for i := range prefix {
		if argv[i] != prefix[i] {
			return false
		}
	}
	return true
}

// HostOK reports whether host matches pattern: exact, "*", or "*.example.com" (subdomains only).
func HostOK(host, pattern string) bool {
	host, pattern = strings.ToLower(host), strings.ToLower(pattern)
	if pattern == "*" || host == pattern {
		return true
	}
	if suf, ok := strings.CutPrefix(pattern, "*."); ok {
		return strings.HasSuffix(host, "."+suf)
	}
	return false
}

// levelRank orders sandbox levels; desktop sits beside workspace-write below unsandboxed.
var levelRank = map[string]int{"read-only": 1, "workspace-write": 2, "desktop": 2, "unsandboxed": 3}

// LevelWithin reports whether level g is no wider than r.
func LevelWithin(g, r string) bool {
	if g == r {
		return true
	}
	if (g == "desktop") != (r == "desktop") && r != "unsandboxed" {
		return false
	}
	return levelRank[g] > 0 && levelRank[g] <= levelRank[r]
}

// Hours parses "<N>h" durations.
func Hours(d string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSuffix(d, "h"))
	return n, err == nil && strings.HasSuffix(d, "h") && n > 0
}

// DurationWithin reports whether duration g is no longer than r (task and wall-clock are not comparable).
func DurationWithin(g, r string) bool {
	if g == r || g == "once" || r == "until_revoked" {
		return true
	}
	gh, ok1 := Hours(g)
	rh, ok2 := Hours(r)
	return ok1 && ok2 && gh <= rh
}

func deepEq(a, b any) bool { return reflect.DeepEqual(a, b) }
