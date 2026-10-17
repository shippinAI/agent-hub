// Package policy evaluates data-defined allow/deny rules. Deny by default; at equal priority deny wins.
package policy

import (
	"slices"
	"sort"
	"strings"
)

// Rule mirrors a row of the policies table.
type Rule struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Subject  string   `json:"subject"`
	Action   string   `json:"action"`
	Selector Selector `json:"selector"`
	Effect   string   `json:"effect"`
	Priority int      `json:"priority"`
	Enabled  bool     `json:"enabled"`
	Version  int64    `json:"version"`
}

// Selector narrows a rule to tasks; empty fields match anything.
type Selector struct {
	Projects []string `json:"projects,omitempty"`
	Labels   []string `json:"labels,omitempty"`
}

// Subject is who is acting.
type Subject struct {
	ID      string
	Kind    string
	IsHuman bool
	Roles   []string
}

// Resource is what the action is on (empty for admin actions).
type Resource struct {
	Project string
	Labels  []string
}

// Decision says whether the action is allowed and which rule decided it.
type Decision struct {
	Allowed bool   `json:"allowed"`
	Rule    string `json:"rule"`
}

// Decide returns the outcome of the highest-priority matching rules.
func Decide(rules []Rule, sub Subject, action string, res Resource) Decision {
	var hits []Rule
	for _, r := range rules {
		if r.Enabled && subjectMatch(r.Subject, sub) && Glob(r.Action, action) && selectorMatch(r.Selector, res) {
			hits = append(hits, r)
		}
	}
	if len(hits) == 0 {
		return Decision{Allowed: false, Rule: "default-deny"}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Priority > hits[j].Priority })
	top := hits[0].Priority
	for _, r := range hits {
		if r.Priority == top && r.Effect == "deny" {
			return Decision{Allowed: false, Rule: r.Name}
		}
	}
	return Decision{Allowed: hits[0].Effect == "allow", Rule: hits[0].Name}
}

func subjectMatch(s string, sub Subject) bool {
	switch {
	case s == "*":
		return true
	case s == "human":
		return sub.IsHuman
	case s == "agent":
		return !sub.IsHuman
	case strings.HasPrefix(s, "role:"):
		return slices.Contains(sub.Roles, s[5:])
	case strings.HasPrefix(s, "principal:"):
		return sub.ID == s[10:]
	case strings.HasPrefix(s, "kind:"):
		return sub.Kind == s[5:]
	}
	return false
}

func selectorMatch(sel Selector, res Resource) bool {
	if len(sel.Projects) > 0 && !slices.Contains(sel.Projects, "*") && !slices.Contains(sel.Projects, res.Project) {
		return false
	}
	for _, l := range sel.Labels {
		if !slices.Contains(res.Labels, l) {
			return false
		}
	}
	return true
}

// Glob matches '*' against any run of characters (including ':' and '>').
func Glob(pattern, s string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == s
	}
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	s = s[len(parts[0]):]
	for i := 1; i < len(parts)-1; i++ {
		k := strings.Index(s, parts[i])
		if k < 0 {
			return false
		}
		s = s[k+len(parts[i]):]
	}
	return strings.HasSuffix(s, parts[len(parts)-1])
}
