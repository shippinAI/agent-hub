package markdown

import "regexp"

// Span is a run of inline text with marks.
type Span struct {
	Text   string
	Bold   bool
	Italic bool
	Code   bool
}

var reInline = regexp.MustCompile("\\*\\*([^*]+)\\*\\*|__([^_]+)__|\\*([^*]+)\\*|_([^_]+)_|`([^`]+)`|\\[([^\\]]+)\\]\\([^)]*\\)")

// Spans splits text into marked runs (links keep their label).
func Spans(s string) []Span {
	var out []Span
	last := 0
	for _, m := range reInline.FindAllStringSubmatchIndex(s, -1) {
		if m[0] > last {
			out = append(out, Span{Text: s[last:m[0]]})
		}
		g := func(i int) string {
			if m[2*i] < 0 {
				return ""
			}
			return s[m[2*i]:m[2*i+1]]
		}
		switch {
		case g(1) != "" || g(2) != "":
			out = append(out, Span{Text: g(1) + g(2), Bold: true})
		case g(3) != "" || g(4) != "":
			out = append(out, Span{Text: g(3) + g(4), Italic: true})
		case g(5) != "":
			out = append(out, Span{Text: g(5), Code: true})
		default:
			out = append(out, Span{Text: g(6)})
		}
		last = m[1]
	}
	if last < len(s) {
		out = append(out, Span{Text: s[last:]})
	}
	return out
}

// Plain drops the marks.
func Plain(s string) string {
	out := ""
	for _, sp := range Spans(s) {
		out += sp.Text
	}
	return out
}
