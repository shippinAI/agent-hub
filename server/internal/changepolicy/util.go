package changepolicy

import (
	"strconv"
	"strings"
)

func firstOr(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return strings.TrimSpace(s)
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }
