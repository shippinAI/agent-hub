package providers

import (
	"context"
	"encoding/json"

	"agenthub/internal/store"
)

// Service is the providers API over the hub store (REST and MCP share it).
type Service struct{ St *store.Store }

// List returns every row ordered by label; enabledOnly hides disabled rows.
func (s *Service) List(ctx context.Context, enabledOnly bool) ([]json.RawMessage, error) {
	rows, err := s.St.DB.Query(ctx, selectRow+` WHERE NOT $1 OR t.disabled_at IS NULL ORDER BY lower(t.label), t.id`, enabledOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// Model is one cached model of a provider (provider_models).
type Model struct {
	Model     string          `json:"model"`
	Label     string          `json:"label"`
	FetchedAt string          `json:"fetched_at"`
	Raw       json.RawMessage `json:"raw"`
}

// Get returns one row plus its model cache.
func (s *Service) Get(ctx context.Context, id string) (map[string]any, error) {
	var b []byte
	if err := s.St.DB.QueryRow(ctx, selectRow+` WHERE t.id=$1`, id).Scan(&b); err != nil {
		return nil, store.ErrNotFound("provider " + id)
	}
	out := map[string]any{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	rows, err := s.St.DB.Query(ctx, `SELECT model, label, to_char(fetched_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'), raw
		FROM provider_models WHERE provider_id=$1 ORDER BY position, model`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	models := []Model{}
	for rows.Next() {
		var m Model
		if err := rows.Scan(&m.Model, &m.Label, &m.FetchedAt, &m.Raw); err != nil {
			return nil, err
		}
		models = append(models, m)
	}
	out["model_cache"] = models
	return out, rows.Err()
}

// Check validates a row without saving it (the validate tool and the UI's live check).
func (s *Service) Check(ctx context.Context, body map[string]json.RawMessage) (any, error) {
	strip(body)
	raw, _ := json.Marshal(body)
	_, errs := Parse(raw)
	if errs == nil {
		errs = Errs{}
	}
	return map[string]any{"ok": len(errs) == 0, "errors": errs}, nil
}
