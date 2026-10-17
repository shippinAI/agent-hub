package store

import (
	"sort"

	"agenthub/internal/policy"
)

func sortRules(rs []policy.Rule) {
	sort.SliceStable(rs, func(i, j int) bool {
		if rs[i].Priority != rs[j].Priority {
			return rs[i].Priority > rs[j].Priority
		}
		return rs[i].Name < rs[j].Name
	})
}
