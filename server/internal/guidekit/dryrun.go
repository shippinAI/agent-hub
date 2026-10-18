package guidekit

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
