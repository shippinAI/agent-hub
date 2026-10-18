// Package docx reads, edits and creates .docx files with the standard library: a text outline, the
// pretty-printed document XML for paging and exact find/replace (DC's DOCX mode, but hash-checked and
// exact-only), and creation from markdown.
package docx

import (
	"regexp"
	"strings"
)

var reToken = regexp.MustCompile(`(?s)<[^>]+>|[^<]+`)

// Pretty puts each element on its own line with indentation, but never adds whitespace inside an element
// that holds text or is empty, so Compact(Pretty(x)) keeps every text node byte for byte.
func Pretty(x string) string {
	toks := reToken.FindAllString(x, -1)
	var b strings.Builder
	depth := 0
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if !strings.HasPrefix(t, "<") {
			if strings.TrimSpace(t) == "" {
				continue
			}
			b.WriteString(strings.Repeat("  ", depth) + t + "\n")
			continue
		}
		isClose, isSelf := strings.HasPrefix(t, "</"), strings.HasSuffix(t, "/>") || strings.HasPrefix(t, "<?")
		if isClose {
			depth = max(0, depth-1)
			b.WriteString(strings.Repeat("  ", depth) + t + "\n")
			continue
		}
		if !isSelf && i+2 < len(toks) && !strings.HasPrefix(toks[i+1], "<") && strings.HasPrefix(toks[i+2], "</") {
			b.WriteString(strings.Repeat("  ", depth) + t + toks[i+1] + toks[i+2] + "\n")
			i += 2
			continue
		}
		if !isSelf && i+1 < len(toks) && strings.HasPrefix(toks[i+1], "</") {
			b.WriteString(strings.Repeat("  ", depth) + t + toks[i+1] + "\n")
			i++
			continue
		}
		b.WriteString(strings.Repeat("  ", depth) + t + "\n")
		if !isSelf {
			depth++
		}
	}
	return b.String()
}

// Compact reverses Pretty: drop the indentation and line breaks it added.
func Compact(p string) string {
	var b strings.Builder
	for _, l := range strings.Split(p, "\n") {
		t := strings.TrimLeft(l, " ")
		if strings.HasPrefix(t, "<") {
			b.WriteString(t)
		} else if t != "" {
			b.WriteString(l[len(l)-len(t):])
		}
	}
	return b.String()
}
