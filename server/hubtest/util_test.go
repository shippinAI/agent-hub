package hubtest

import (
	"strconv"
	"strings"
)

func itoa(i int64) string                  { return strconv.FormatInt(i, 10) }
func replaceAll(s, old, new string) string { return strings.ReplaceAll(s, old, new) }
