package guidekit

import (
	"fmt"
	"regexp"
)

// SecretNote flags notes that look like they carry a credential.
var SecretNote = regexp.MustCompile(`(?i)(api[_-]?key|password|secret|token)\s*[:=]\s*\S{6,}`)

var placeholder = regexp.MustCompile(`\{\{\s*([a-z_]+(?:\.[a-z0-9_]+)?)\s*\}\}`)

// Fill replaces {{hub_url}} and {{vars.x}} in s; unknown names stay as written so a reader sees the gap.
func Fill(s string, vals map[string]any) string {
	return placeholder.ReplaceAllStringFunc(s, func(m string) string {
		k := placeholder.FindStringSubmatch(m)[1]
		if v, ok := vals[k]; ok {
			return fmt.Sprint(v)
		}
		return m
	})
}

// Render picks the text and command one OS sees for a step and fills placeholders. commands resolves a
// command_ref (runtime preset operator command) for that OS.
func Render(s Step, os string, vals map[string]any, commands func(runtime, key, os string) string) map[string]any {
	text := s.Text
	if t := s.TextByOS[os]; t != "" {
		text = t
	}
	cmd := s.CommandByOS[os]
	if s.CommandRef != nil && commands != nil {
		cmd = commands(s.CommandRef.Runtime, s.CommandRef.Key, os)
	}
	out := map[string]any{"id": s.ID, "kind": s.Kind, "actor": s.Actor, "title": Fill(s.Title, vals), "text": Fill(text, vals)}
	if cmd != "" {
		out["command"] = Fill(cmd, vals)
	}
	for k, v := range map[string]string{"changes": s.Changes, "check": s.Check, "question": s.Question, "reason": s.Reason} {
		if v != "" {
			out[k] = Fill(v, vals)
		}
	}
	if len(s.Options) > 0 {
		out["options"] = s.Options
	}
	if len(s.RiskIDs) > 0 {
		out["risk_ids"] = s.RiskIDs
	}
	return out
}
