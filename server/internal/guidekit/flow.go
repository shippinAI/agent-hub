package guidekit

import (
	"fmt"
	"strings"
)

// Ctx is what branch conditions see: the machine profile (os, arch, display_server, kind) and run vars.
type Ctx struct {
	Profile map[string]any
	Vars    map[string]any
	Outcome string
}

func (c Ctx) lookup(key string) any {
	if key == "outcome" {
		return c.Outcome
	}
	if v, ok := strings.CutPrefix(key, "vars."); ok {
		return c.Vars[v]
	}
	return c.Profile[key]
}

// Match reports whether every key of when matches (a list value means "any of").
func (c Ctx) Match(when map[string]any) bool {
	for k, want := range when {
		got := fmt.Sprint(c.lookup(k))
		switch w := want.(type) {
		case []any:
			hit := false
			for _, x := range w {
				hit = hit || fmt.Sprint(x) == got
			}
			if !hit {
				return false
			}
		default:
			if fmt.Sprint(w) != got {
				return false
			}
		}
	}
	return true
}

// after is the step that follows i in order ("" past the end).
func after(steps []Step, id string) string {
	for i := range steps {
		if steps[i].ID == id && i+1 < len(steps) {
			return steps[i+1].ID
		}
	}
	return ""
}

func target(steps []Step, s *Step, c Ctx) string {
	for _, cs := range s.Cases {
		if c.Match(cs.When) {
			return cs.Goto
		}
	}
	if s.Default != "" {
		return s.Default
	}
	if s.Next != "" {
		return s.Next
	}
	return after(steps, s.ID)
}

// Settle walks through branch steps from id until a step that needs someone to act ("" = the guide ended).
func Settle(steps []Step, id string, c Ctx) string {
	for hops := 0; id != "" && hops < len(steps)+1; hops++ {
		s := Find(steps, id)
		if s == nil || s.Kind != "branch" {
			return id
		}
		c.Outcome = ""
		id = target(steps, s, c)
	}
	return id
}

// Result is where a run goes after an outcome.
type Result struct {
	Next   string `json:"next"`
	Done   bool   `json:"done"`
	Paused bool   `json:"paused"`
	Retry  int    `json:"retry"`
}

// Advance computes the next step after cur finished with outcome; retries counts earlier failures of cur.
func Advance(steps []Step, cur, outcome string, retries int, c Ctx) (Result, error) {
	s := Find(steps, cur)
	if s == nil {
		return Result{}, fmt.Errorf("unknown step %q", cur)
	}
	if s.Kind == "confirm" && outcome != "skip" && !hasOption(s, outcome) {
		return Result{}, fmt.Errorf("step %q expects one of its options", cur)
	}
	if outcome == "skip" && !s.Skippable && s.Kind != "explain" {
		return Result{}, fmt.Errorf("step %q cannot be skipped", cur)
	}
	c.Outcome = outcome
	for _, cs := range s.Cases {
		if c.Match(cs.When) {
			return Result{Next: Settle(steps, cs.Goto, c)}, nil
		}
	}
	if outcome == "fail" && (s.OnFail != "" || s.MaxRetries > 0) {
		if retries+1 > s.MaxRetries {
			return Result{Next: cur, Paused: true, Retry: retries + 1}, nil
		}
		nxt := cur
		if s.OnFail != "" {
			nxt = s.OnFail
		}
		return Result{Next: Settle(steps, nxt, c), Retry: retries + 1}, nil
	}
	if outcome == "fail" && s.Kind == "verify" {
		return Result{Next: cur, Paused: true, Retry: retries + 1}, nil
	}
	if s.End {
		return Result{Done: true}, nil
	}
	nxt := Settle(steps, target(steps, s, c), c)
	return Result{Next: nxt, Done: nxt == ""}, nil
}

func hasOption(s *Step, id string) bool {
	for _, o := range s.Options {
		if o.ID == id {
			return true
		}
	}
	return false
}

// DryRun resolves a path through the guide without touching any machine: every agent step is assumed to
// return assume[step] (default "ok"), and confirm steps take choices[step] or their first option.
func DryRun(steps []Step, profile map[string]any, choices, assume map[string]string) ([]string, error) {
	c := Ctx{Profile: profile, Vars: map[string]any{}}
	path := []string{}
	cur := Settle(steps, steps[0].ID, c)
	for hops := 0; cur != "" && hops < 4*len(steps); hops++ {
		path = append(path, cur)
		s := Find(steps, cur)
		out := "ok"
		if v, ok := assume[cur]; ok {
			out = v
		}
		if s.Kind == "confirm" {
			out = s.Options[0].ID
			if v, ok := choices[cur]; ok {
				out = v
			}
		}
		if s.SaveAs != "" {
			c.Vars[s.SaveAs] = out
		}
		r, err := Advance(steps, cur, out, 0, c)
		if err != nil || r.Done || r.Paused {
			return path, err
		}
		cur = r.Next
	}
	return path, nil
}
