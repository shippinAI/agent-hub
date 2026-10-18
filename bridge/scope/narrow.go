package scope

import "fmt"

// SelectorWithin reports whether selector g is no wider than r; the error names the first widened key.
// Lists mean "only these" except where noted: an absent paths/argv_prefix/actions/apps list means "any",
// an absent hosts/env_allow list means "none".
func SelectorWithin(g, r Sel) error {
	for k := range g {
		if _, ok := r[k]; ok || anyWhenAbsent[k] || limitKey[k] {
			continue
		}
		if l, isList := g.Strs(k); isList && len(l) == 0 && (k == "hosts" || k == "env_allow") {
			continue
		}
		return fmt.Errorf("%s was not requested", k)
	}
	for k, rv := range r {
		gv, has := g[k]
		switch {
		case !has && (k == "hosts" || k == "env_allow" || k == "deny_keys"):
			if k == "deny_keys" {
				return fmt.Errorf("deny_keys dropped")
			}
			continue
		case !has && limitKey[k]:
			return fmt.Errorf("%s limit removed", k)
		case !has:
			return fmt.Errorf("%s removed (would mean any)", k)
		}
		if err := keyWithin(k, g, r, gv, rv); err != nil {
			return err
		}
	}
	return nil
}

var anyWhenAbsent = map[string]bool{"paths": true, "argv_prefix": true, "actions": true, "apps": true, "windows": true, "monitors": true}
var limitKey = map[string]bool{"max_runtime_s": true, "max_actions_per_minute": true, "max_edge": true, "tree_depth": true}

func keyWithin(k string, g, r Sel, gv, rv any) error {
	switch {
	case limitKey[k]:
		gn, _ := g.Num(k)
		rn, _ := r.Num(k)
		if gn > rn {
			return fmt.Errorf("%s raised", k)
		}
	case k == "argv_prefix":
		for _, gp := range g.Prefixes() {
			if !anyPrefix(gp, r.Prefixes()) {
				return fmt.Errorf("argv_prefix %v not within the request", gp)
			}
		}
	case k == "deny_keys":
		gl, _ := g.Strs(k)
		rl, _ := r.Strs(k)
		for _, x := range rl {
			if !contains(gl, x) {
				return fmt.Errorf("deny_keys dropped %s", x)
			}
		}
	case r.Any(k):
		return nil
	case g.Any(k):
		return fmt.Errorf("%s widened to any", k)
	default:
		gl, ok1 := g.Strs(k)
		rl, ok2 := r.Strs(k)
		if !ok1 || !ok2 {
			if !deepEq(gv, rv) {
				return fmt.Errorf("%s changed", k)
			}
			return nil
		}
		for _, x := range gl {
			if !listHas(k, rl, x) {
				return fmt.Errorf("%s %q not within the request", k, x)
			}
		}
	}
	return nil
}

func listHas(k string, l []string, x string) bool {
	for _, y := range l {
		if (k == "paths" && Within(x, y)) || (k == "hosts" && HostOK(trimStar(x), y)) || x == y {
			return true
		}
	}
	return false
}

func trimStar(h string) string {
	if len(h) > 2 && h[:2] == "*." {
		return "a" + h[1:]
	}
	return h
}

func anyPrefix(p []string, l [][]string) bool {
	for _, q := range l {
		if HasPrefix(p, q) {
			return true
		}
	}
	return false
}

func contains(l []string, x string) bool {
	for _, y := range l {
		if x == y {
			return true
		}
	}
	return false
}
