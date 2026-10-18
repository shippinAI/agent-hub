package providers

import (
	"encoding/json"
	"fmt"
	"strings"

	"agenthub/internal/store"
)

// SeedFile is a presets/devagotchi/provider-*.json file.
type SeedFile struct {
	Rows []json.RawMessage `json:"rows"`
}

// Seed validates and inserts the preset rows once (marked in preset_marks), never overwriting a row an
// operator already has. A preset row that fails validation stops the seed: shipped data must be valid.
func Seed(w *store.W, mark string, rows []json.RawMessage, models []json.RawMessage) (int, error) {
	tag, err := w.Exec(`INSERT INTO preset_marks(name) VALUES($1) ON CONFLICT DO NOTHING`, mark)
	if err != nil || tag.RowsAffected() == 0 {
		return 0, err
	}
	n := 0
	for i, raw := range rows {
		p, errs := Parse(raw)
		if len(errs) == 0 {
			errs = routers(w, p)
		}
		if len(errs) > 0 {
			return n, fmt.Errorf("preset provider row %d (%s): %v", i, p.ID, errs)
		}
		c, err := insert(w, p, true)
		if err != nil {
			return n, fmt.Errorf("preset provider %s: %w", p.ID, err)
		}
		n += int(c)
	}
	byProvider := map[string][]ModelIn{}
	for _, raw := range models {
		var m struct {
			ProviderID string `json:"provider_id"`
			ModelIn
		}
		if err := json.Unmarshal(raw, &m); err != nil || m.ProviderID == "" || m.Model == "" {
			return n, fmt.Errorf("preset provider model: needs provider_id and model")
		}
		byProvider[m.ProviderID] = append(byProvider[m.ProviderID], m.ModelIn)
	}
	for id, list := range byProvider {
		if err := putModels(w, id, list, "preset"); err != nil {
			return n, err
		}
	}
	return n, nil
}

func putModels(w *store.W, id string, list []ModelIn, source string) error {
	for i, m := range list {
		if strings.TrimSpace(m.Model) == "" || len(m.Model) > 200 {
			return store.ErrInvalid("models[%d]: model name required (max 200 chars)", i)
		}
		if _, err := w.Exec(`INSERT INTO provider_models(provider_id, model, label, position, raw)
			SELECT $1, $2, $3, $4, jsonb_build_object('source', $5::text) WHERE EXISTS (SELECT 1 FROM providers WHERE id=$1)
			ON CONFLICT DO NOTHING`, id, m.Model, m.Label, i, source); err != nil {
			return err
		}
	}
	return nil
}
