package procs

import (
	"regexp"
	"strings"
)

// Prompts recognises "waiting for input" from the text after the last newline. Patterns are policy data.
type Prompts struct{ res []*regexp.Regexp }

// NewPrompts compiles the policy's prompt_patterns, skipping invalid ones (reported by doctor).
func NewPrompts(patterns []string) Prompts {
	p := Prompts{}
	for _, s := range patterns {
		if re, err := regexp.Compile(s); err == nil {
			p.res = append(p.res, re)
		}
	}
	return p
}

// Match reports whether tail looks like a prompt. ANSI colour codes are ignored.
func (p Prompts) Match(tail string) bool {
	tail = stripANSI(tail)
	if strings.TrimSpace(tail) == "" {
		return false
	}
	for _, re := range p.res {
		if re.MatchString(tail) {
			return true
		}
	}
	return false
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07]*\x07|\r`)

func stripANSI(s string) string { return ansi.ReplaceAllString(s, "") }

// Invalid lists patterns that do not compile (for doctor).
func Invalid(patterns []string) []string {
	var bad []string
	for _, s := range patterns {
		if _, err := regexp.Compile(s); err != nil {
			bad = append(bad, s)
		}
	}
	return bad
}
