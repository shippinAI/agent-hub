// Package redact removes secret-looking values from anything the bridge logs.
package redact

import (
	"regexp"
	"strings"
)

// Defaults are always applied (ahk_, sk- and other known prefixes); the hub's secret_patterns add more.
var Defaults = []string{"ahk_", "sk-", "xai-", "ghp_", "github_pat_", "AKIA"}

// Redactor masks values that start with a known prefix.
type Redactor struct{ re *regexp.Regexp }

// New builds a redactor from prefixes.
func New(prefixes []string) Redactor {
	all := append(append([]string{}, Defaults...), prefixes...)
	q := make([]string, 0, len(all))
	for _, p := range all {
		if p = strings.TrimSpace(p); p != "" {
			q = append(q, regexp.QuoteMeta(p))
		}
	}
	return Redactor{re: regexp.MustCompile(`(` + strings.Join(q, "|") + `)[A-Za-z0-9_\-\.]{6,}`)}
}

// String masks secrets in s, keeping the prefix so the kind stays visible.
func (r Redactor) String(s string) string { return r.re.ReplaceAllString(s, "${1}[redacted]") }
