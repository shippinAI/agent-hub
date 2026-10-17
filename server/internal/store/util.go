package store

import (
	"strconv"
	"strings"
)

func itoa(i int) string { return strconv.Itoa(i) }

func ptr[T any](v T) *T { return &v }

func cleanLabels(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, l := range in {
		l = strings.TrimSpace(strings.ToLower(l))
		if l != "" && !seen[l] && len(l) <= 64 {
			seen[l] = true
			out = append(out, l)
		}
	}
	return out
}
