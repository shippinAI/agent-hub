package guidekit

import (
	"encoding/json"
	"reflect"
)

// Change is one step-level difference between two versions, keyed by step id.
type Change struct {
	Step   string          `json:"step"`
	Change string          `json:"change"`
	Before json.RawMessage `json:"before,omitempty"`
	After  json.RawMessage `json:"after,omitempty"`
}

// Diff compares two step lists by step id: added, removed, changed and moved.
func Diff(a, b []Step) []Change {
	out := []Change{}
	pos := func(s []Step) map[string]int {
		m := map[string]int{}
		for i, x := range s {
			m[x.ID] = i
		}
		return m
	}
	pa, pb := pos(a), pos(b)
	js := func(s Step) json.RawMessage { r, _ := json.Marshal(s); return r }
	for _, s := range a {
		j, ok := pb[s.ID]
		switch {
		case !ok:
			out = append(out, Change{Step: s.ID, Change: "removed", Before: js(s)})
		case !reflect.DeepEqual(s, b[j]):
			out = append(out, Change{Step: s.ID, Change: "changed", Before: js(s), After: js(b[j])})
		case pa[s.ID] != j:
			out = append(out, Change{Step: s.ID, Change: "moved"})
		}
	}
	for _, s := range b {
		if _, ok := pa[s.ID]; !ok {
			out = append(out, Change{Step: s.ID, Change: "added", After: js(s)})
		}
	}
	return out
}
