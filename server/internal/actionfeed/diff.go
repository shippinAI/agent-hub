package actionfeed

import (
	"fmt"
	"strings"
)

// DiffFromArgs renders a unified-style diff for edit-shaped arguments (old_string/new_string as used by
// edit_block, or path + content for whole-file writes). Other shapes return "".
func DiffFromArgs(a map[string]any) string {
	if a == nil {
		return ""
	}
	path := first(a, "file_path", "path")
	oldS, hasOld := a["old_string"].(string)
	newS, hasNew := a["new_string"].(string)
	if !hasOld && !hasNew {
		content, ok := a["content"].(string)
		if !ok || path == "" {
			return ""
		}
		mode := first(a, "mode")
		return fmt.Sprintf("--- %s\n+++ %s (%s)\n%s", path, path, orDefault(mode, "rewrite"), prefix(content, "+"))
	}
	rel := strings.TrimLeft(path, "/")
	return fmt.Sprintf("--- a/%s\n+++ b/%s\n@@ edit @@\n%s%s", rel, rel, prefix(oldS, "-"), prefix(newS, "+"))
}

func prefix(s, p string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = p + l
	}
	return strings.Join(lines, "\n") + "\n"
}

func first(a map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := a[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}
