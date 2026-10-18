package guidekit

import (
	"fmt"
	"regexp"
	"strings"
)

// Finding is one lint result; Block findings stop a publish.
type Finding struct {
	Step    string `json:"step"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

var secretish = regexp.MustCompile(`(?i)(api[_-]?key|password|secret|token)\s*[:=]\s*\S{6,}|sk-[A-Za-z0-9]{16,}|ghp_[A-Za-z0-9]{20,}`)

// Lint checks what the schema cannot: per-OS coverage of operator steps on every OS that can reach them,
// risky steps without risk ids, secret-looking text, and dead steps.
func Lint(steps []Step, oses []string, extra []*regexp.Regexp) []Finding {
	out := []Finding{}
	add := func(s, lvl, f string, a ...any) { out = append(out, Finding{s, lvl, fmt.Sprintf(f, a...)}) }
	reach := map[string][]string{}
	for _, os := range oses {
		for id := range Reachable(steps, os) {
			reach[id] = append(reach[id], os)
		}
	}
	for _, s := range steps {
		if _, ok := reach[s.ID]; !ok && len(oses) > 0 {
			add(s.ID, "warn", "step is not reachable on any listed OS")
		}
		if s.Kind == "operator_action" && s.CommandRef == nil {
			for _, os := range reach[s.ID] {
				if s.Text == "" && s.TextByOS[os] == "" {
					add(s.ID, "block", "operator step has no text for %s", os)
				}
				if len(s.CommandByOS) > 0 && s.CommandByOS[os] == "" {
					add(s.ID, "block", "operator step has no command for %s", os)
				}
			}
		}
		if (s.Kind == "agent_action" || s.Kind == "check" || s.Kind == "verify") && s.Op == "" {
			add(s.ID, "block", "agent step needs an op")
		}
		body := strings.Join([]string{s.Text, s.Changes, s.Check, fmt.Sprint(s.Args), fmt.Sprint(s.CommandByOS), fmt.Sprint(s.TextByOS)}, " ")
		if secretish.MatchString(body) {
			add(s.ID, "block", "step text looks like it holds a secret")
		}
		for _, re := range extra {
			if re.MatchString(body) {
				add(s.ID, "block", "step text matches a secret pattern")
			}
		}
		if s.Kind == "operator_action" && strings.Contains(strings.Join(mapVals(s.CommandByOS), " "), "sudo") && len(s.RiskIDs) == 0 && s.Changes == "" {
			add(s.ID, "warn", "admin command without risk ids or a changes note")
		}
	}
	return out
}

func mapVals(m map[string]string) []string {
	v := []string{}
	for _, x := range m {
		v = append(v, x)
	}
	return v
}

// Blocking reports whether any finding blocks publishing.
func Blocking(f []Finding) bool {
	for _, x := range f {
		if x.Level == "block" {
			return true
		}
	}
	return false
}

// Reachable walks every path a machine with this OS can take. A branch case whose only key is os is taken
// exactly (it matches or it does not); any other case may or may not match, so both sides are explored.
func Reachable(steps []Step, os string) map[string]bool {
	seen := map[string]bool{}
	var walk func(id string)
	walk = func(id string) {
		if id == "" || seen[id] || Find(steps, id) == nil {
			return
		}
		seen[id] = true
		s := Find(steps, id)
		exact := false
		for _, c := range s.Cases {
			if v, ok := c.When["os"]; ok && len(c.When) == 1 {
				if (Ctx{Profile: map[string]any{"os": os}}).Match(map[string]any{"os": v}) {
					walk(c.Goto)
					exact = true
					break
				}
				continue
			}
			walk(c.Goto)
		}
		if exact && s.Kind == "branch" {
			return
		}
		if s.OnFail != "" {
			walk(s.OnFail)
		}
		if s.End {
			return
		}
		switch {
		case s.Default != "":
			walk(s.Default)
		case s.Next != "":
			walk(s.Next)
		default:
			walk(after(steps, s.ID))
		}
	}
	if len(steps) > 0 {
		walk(steps[0].ID)
	}
	return seen
}
