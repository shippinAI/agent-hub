package access

import (
	"fmt"

	"agenthub/internal/scope"
)

func firstOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func hours(d string) int {
	h, _ := scope.Hours(d)
	return h
}

func errf(f string, a ...any) error { return fmt.Errorf(f, a...) }
