// Package guidekit is the pure logic of guides: parse, validate, lint, run flow, dry-run, diff and signing.
// Guide text lives in data (guide_versions.steps); nothing here knows any particular guide.
package guidekit

import (
	"encoding/json"
	"fmt"

	"agenthub/internal/schemacheck"
)

// Step is one guide step (see docs/devagotchi/12-guides.md#step-kinds).
type Step struct {
	ID           string            `json:"id"`
	Kind         string            `json:"kind"`
	Actor        string            `json:"actor,omitempty"`
	Title        string            `json:"title"`
	Text         string            `json:"text,omitempty"`
	TextByOS     map[string]string `json:"text_by_os,omitempty"`
	CommandByOS  map[string]string `json:"command_by_os,omitempty"`
	CommandRef   *Ref              `json:"command_ref,omitempty"`
	Changes      string            `json:"changes,omitempty"`
	Check        string            `json:"check,omitempty"`
	RiskIDs      []string          `json:"risk_ids,omitempty"`
	Op           string            `json:"op,omitempty"`
	Args         map[string]any    `json:"args,omitempty"`
	ExpectExit   *int              `json:"expect_exit,omitempty"`
	Preset       string            `json:"preset,omitempty"`
	Capability   string            `json:"capability,omitempty"`
	Selector     map[string]any    `json:"selector,omitempty"`
	SandboxLevel string            `json:"sandbox_level,omitempty"`
	Duration     string            `json:"duration,omitempty"`
	Reason       string            `json:"reason,omitempty"`
	Question     string            `json:"question,omitempty"`
	Options      []Option          `json:"options,omitempty"`
	Guide        string            `json:"guide,omitempty"`
	Cases        []Case            `json:"cases,omitempty"`
	Default      string            `json:"default,omitempty"`
	Next         string            `json:"next,omitempty"`
	OnFail       string            `json:"on_fail,omitempty"`
	MaxRetries   int               `json:"max_retries,omitempty"`
	SaveAs       string            `json:"save_as,omitempty"`
	Skippable    bool              `json:"skippable,omitempty"`
	End          bool              `json:"end,omitempty"`
}

// Ref points an operator step at a command stored in a container runtime preset.
type Ref struct {
	Runtime string `json:"runtime"`
	Key     string `json:"key"`
}

// Option is one answer of a confirm step.
type Option struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Case jumps to Goto when every key of When matches.
type Case struct {
	When map[string]any `json:"when"`
	Goto string         `json:"goto"`
}

// Parse decodes a steps document.
func Parse(raw []byte) ([]Step, error) {
	var s []Step
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("steps: %w", err)
	}
	return s, nil
}

// Validate checks the steps against the stored JSON Schema plus the references the schema cannot express.
func Validate(schema, raw []byte) ([]Step, error) {
	if err := schemacheck.Validate(schema, raw); err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	steps, err := Parse(raw)
	if err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	for _, s := range steps {
		if ids[s.ID] {
			return nil, fmt.Errorf("duplicate step id %q", s.ID)
		}
		ids[s.ID] = true
	}
	for _, s := range steps {
		targets := []string{s.Default, s.Next, s.OnFail}
		for _, c := range s.Cases {
			targets = append(targets, c.Goto)
		}
		for _, t := range targets {
			if t != "" && !ids[t] {
				return nil, fmt.Errorf("step %q points to unknown step %q", s.ID, t)
			}
		}
		if s.Kind == "branch" && len(s.Cases) == 0 && s.Default == "" {
			return nil, fmt.Errorf("branch %q needs cases or a default", s.ID)
		}
		if s.Kind == "link_guide" && s.Guide == "" {
			return nil, fmt.Errorf("link_guide %q needs guide", s.ID)
		}
		if s.Kind == "confirm" && len(s.Options) == 0 {
			return nil, fmt.Errorf("confirm %q needs options", s.ID)
		}
	}
	return steps, nil
}

// Find returns the step with id, or nil.
func Find(steps []Step, id string) *Step {
	for i := range steps {
		if steps[i].ID == id {
			return &steps[i]
		}
	}
	return nil
}
