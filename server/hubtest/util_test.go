package hubtest

import (
	"strconv"
	"strings"
)

func itoa(i int64) string                  { return strconv.FormatInt(i, 10) }
func replaceAll(s, old, new string) string { return strings.ReplaceAll(s, old, new) }

func errCode(out map[string]any) string {
	if e, ok := out["error"].(map[string]any); ok {
		s, _ := e["code"].(string)
		return s
	}
	if s, ok := out["error"].(string); ok {
		return s
	}
	return ""
}
