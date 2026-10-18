package presets

import (
	"embed"
	"encoding/json"
	"fmt"

	"agenthub/internal/policy"
	"agenthub/internal/store"
)

//go:embed devagotchi/*.json
var devFS embed.FS

//go:embed guides/*.json
var guideFS embed.FS

// CatalogFile is one devagotchi/*.json file of catalog rows.
type CatalogFile struct {
	Name string
	Kind string                       `json:"kind"`
	Rows []map[string]json.RawMessage `json:"rows"`
}

// Devagotchi is the Devagotchi preset bundle: settings, roles, policies, catalog rows and the Shippin guides.
type Devagotchi struct {
	Settings []store.SettingRow
	Roles    []store.Role
	Policies []policy.Rule
	Catalog  []CatalogFile
	Guides   []json.RawMessage
}

// catalogOrder respects foreign keys (capabilities before links and presets, routers before providers).
var catalogOrder = []string{"capabilities", "risks", "capability-risks", "shell-profiles", "grant-presets", "runtimes", "routers", "providers", "chat-apps"}

// LoadDevagotchi parses the embedded bundle.
func LoadDevagotchi() (Devagotchi, error) {
	var d Devagotchi
	for _, f := range []string{"settings", "step-schema"} {
		var s struct {
			Settings []store.SettingRow `json:"settings"`
		}
		if err := readJSON(devFS, "devagotchi/"+f+".json", &s); err != nil {
			return d, err
		}
		d.Settings = append(d.Settings, s.Settings...)
	}
	var pol struct {
		Roles    []store.Role  `json:"roles"`
		Policies []policy.Rule `json:"policies"`
	}
	if err := readJSON(devFS, "devagotchi/policies.json", &pol); err != nil {
		return d, err
	}
	d.Roles, d.Policies = pol.Roles, pol.Policies
	for i := range d.Policies {
		d.Policies[i].Enabled = true
	}
	for _, n := range catalogOrder {
		cf := CatalogFile{Name: n}
		if err := readJSON(devFS, "devagotchi/"+n+".json", &cf); err != nil {
			return d, err
		}
		d.Catalog = append(d.Catalog, cf)
	}
	ents, err := guideFS.ReadDir("guides")
	if err != nil {
		return d, err
	}
	for _, e := range ents {
		b, err := guideFS.ReadFile("guides/" + e.Name())
		if err != nil {
			return d, err
		}
		d.Guides = append(d.Guides, b)
	}
	return d, nil
}

func readJSON(fs embed.FS, name string, v any) error {
	b, err := fs.ReadFile(name)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("preset %s: %w", name, err)
	}
	return nil
}
