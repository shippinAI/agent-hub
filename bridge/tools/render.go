package tools

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// render prints a map as "key: value" lines (stable order) for the text part of a result.
func render(m map[string]any) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		v := m[k]
		switch v.(type) {
		case string, int, int64, float64, bool:
			fmt.Fprintf(&b, "%s: %v\n", k, v)
		default:
			js, _ := json.Marshal(v)
			fmt.Fprintf(&b, "%s: %s\n", k, js)
		}
	}
	return b.String()
}

func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

func jsonText(v any) (string, error) {
	if s, ok := v.(string); ok {
		return s, nil
	}
	b, err := json.Marshal(v)
	return string(b), err
}
