package tools

import (
	"fmt"
	"strings"
)

// nearMiss finds the window of the file most like old (same line count, longest common prefix+suffix) and
// renders the difference as prefix{-removed-}{+added+}suffix. It only explains; it never edits.
func nearMiss(s, old string) string {
	if len(s) > 4<<20 || old == "" {
		return ""
	}
	lines := strings.SplitAfter(s, "\n")
	n := max(1, strings.Count(strings.TrimSuffix(old, "\n"), "\n")+1)
	norm := strings.Join(strings.Fields(old), " ")
	best, bestScore, bestLine := "", -1, 0
	for i := 0; i+n <= len(lines); i++ {
		w := strings.TrimSuffix(strings.Join(lines[i:i+n], ""), "\n")
		if strings.Join(strings.Fields(w), " ") == norm {
			return fmt.Sprintf("Closest match at line %d differs only in whitespace or indentation:\n%s", i+1, diff(w, old))
		}
		if sc := common(w, old); sc > bestScore {
			best, bestScore, bestLine = w, sc, i+1
		}
	}
	if bestScore < len(old)/4 {
		return "No similar text found; re-read the file to get the exact current content."
	}
	return fmt.Sprintf("Closest text is at line %d ({-file-}{+your old_string+}):\n%s", bestLine, diff(best, old))
}

func common(a, b string) int {
	p := 0
	for p < len(a) && p < len(b) && a[p] == b[p] {
		p++
	}
	q := 0
	for q < len(a)-p && q < len(b)-p && a[len(a)-1-q] == b[len(b)-1-q] {
		q++
	}
	return p + q
}

// diff renders the middle that differs between file text a and wanted text b.
func diff(a, b string) string {
	p := 0
	for p < len(a) && p < len(b) && a[p] == b[p] {
		p++
	}
	q := 0
	for q < len(a)-p && q < len(b)-p && a[len(a)-1-q] == b[len(b)-1-q] {
		q++
	}
	vis := strings.NewReplacer("\t", "\\t", "\r", "\\r")
	return a[:p] + "{-" + vis.Replace(a[p:len(a)-q]) + "-}{+" + vis.Replace(b[p:len(b)-q]) + "+}" + a[len(a)-q:]
}
