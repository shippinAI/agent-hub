package policyrules

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
)

var effects = []string{"require", "deny", "allow", "audit", "collect"}
var kinds = []string{"ticket", "field", "list", "number", "approval", "review", "via_runner"}

// Validate rejects a Doc that would silently mis-enforce (bad regex, unknown effect/check, dup ids).
func (d *Doc) Validate() error {
	for name, pats := range d.Classes {
		for _, p := range pats {
			if _, err := regexp.Compile(p); err != nil {
				return fmt.Errorf("class %s: bad regex %q: %v", name, p, err)
			}
		}
	}
	seen := map[string]bool{}
	for _, r := range d.Rules {
		if err := r.Validate(); err != nil {
			return err
		}
		if seen[r.ID] {
			return fmt.Errorf("duplicate rule id %s", r.ID)
		}
		seen[r.ID] = true
	}
	return nil
}

// Validate checks one rule.
func (r Rule) Validate() error {
	if !ValidID(r.ID) {
		return fmt.Errorf("rule id %q must match [A-Za-z0-9._-]{3,64}", r.ID)
	}
	if !slices.Contains(effects, r.Effect) {
		return fmt.Errorf("rule %s: effect %q must be one of %v", r.ID, r.Effect, effects)
	}
	if r.Effect == "require" && len(r.Require) == 0 {
		return fmt.Errorf("rule %s: effect require needs at least one check", r.ID)
	}
	for i, c := range r.Require {
		if !slices.Contains(kinds, c.Kind) {
			return fmt.Errorf("rule %s check %d: kind %q must be one of %v", r.ID, i, c.Kind, kinds)
		}
		if (c.Kind == "field" || c.Kind == "list" || c.Kind == "number") && c.Field == "" {
			return fmt.Errorf("rule %s check %d: %s needs a field", r.ID, i, c.Kind)
		}
		for _, p := range []string{c.Pattern, c.Not} {
			if _, err := regexp.Compile(p); err != nil {
				return fmt.Errorf("rule %s check %d: bad regex %q", r.ID, i, p)
			}
		}
	}
	for _, p := range append(slices.Clone(r.When.Tools), r.When.CommandRe) {
		if _, err := regexp.Compile(p); err != nil && p != "*" {
			return fmt.Errorf("rule %s: bad regex %q", r.ID, p)
		}
	}
	return nil
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }
