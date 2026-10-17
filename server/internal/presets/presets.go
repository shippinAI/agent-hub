// Package presets embeds the starter data (roles, policies, workflow, settings, principals).
// It is only used to seed an empty hub; afterwards everything is edited as data in the UI.
package presets

import (
	_ "embed"
	"encoding/json"

	"agenthub/internal/policy"
	"agenthub/internal/store"
)

//go:embed default.json
var defaultJSON []byte

// Preset is the shape of default.json.
type Preset struct {
	Settings   []store.SettingRow `json:"settings"`
	Roles      []store.Role       `json:"roles"`
	Principals []store.Principal  `json:"principals"`
	Workflows  []store.Workflow   `json:"workflows"`
	Projects   []store.Project    `json:"projects"`
	Policies   []policy.Rule      `json:"policies"`
}

// Default parses the embedded preset.
func Default() (Preset, error) {
	var p Preset
	err := json.Unmarshal(defaultJSON, &p)
	for i := range p.Policies {
		p.Policies[i].Enabled = true
	}
	return p, err
}
