package providers

import (
	"context"
	"encoding/json"

	"agenthub/internal/store"
)

// Save creates (version 0, id must be new) or updates (version must match) a row. With merge (PATCH) the
// body's top-level fields replace the stored row's; without it (PUT, POST) the body is the whole row.
// Writes are human-only plus the catalog.providers.put policy, because rows carry argv the bridge runs.
func (s *Service) Save(ctx context.Context, p *store.Principal, reqID, id string, body map[string]json.RawMessage, merge bool) (any, error) {
	version := strip(body)
	if id == "" {
		_ = json.Unmarshal(body["id"], &id)
	}
	return s.St.Write(ctx, p, reqID, "provider_save", map[string]any{"id": id, "body": body}, func(w *store.W) (any, error) {
		if err := w.Human("catalog.providers.put"); err != nil {
			return nil, err
		}
		var cur int64
		var stored []byte
		exists := w.Row(`SELECT version, to_jsonb(t) FROM providers t WHERE id=$1 FOR UPDATE`, id).Scan(&cur, &stored) == nil
		switch {
		case version == 0 && exists:
			return nil, store.Fail(409, "exists", "provider %s already exists; send its version to update it", id)
		case version != 0 && !exists:
			return nil, store.ErrNotFound("provider " + id)
		case exists && cur != version:
			return nil, store.ErrVersion(map[string]any{"version": cur})
		}
		if merge && exists {
			base := map[string]json.RawMessage{}
			_ = json.Unmarshal(stored, &base)
			strip(base)
			for k, v := range body {
				base[k] = v
			}
			body = base
		}
		if raw, ok := body["id"]; ok {
			var bid string
			if json.Unmarshal(raw, &bid) != nil || bid != id {
				return nil, Invalid(Errs{{Path: "id", Message: "cannot change; create a new row instead"}})
			}
		}
		body["id"], _ = json.Marshal(id)
		raw, _ := json.Marshal(body)
		row, errs := Parse(raw)
		if len(errs) == 0 {
			errs = routers(w, row)
		}
		if len(errs) > 0 {
			return nil, Invalid(errs)
		}
		var err error
		if exists {
			err = update(w, row)
		} else {
			_, err = insert(w, row, false)
		}
		if err != nil {
			return nil, store.ErrInvalid("%v", err)
		}
		var out json.RawMessage
		if err := w.Row(selectRow+` WHERE t.id=$1`, id).Scan(&out); err != nil {
			return nil, err
		}
		return out, w.Event("provider.saved", nil, nil, map[string]any{"id": id, "created": !exists})
	})
}

// SetDisabled turns a row off (or back on). Disabled rows stay readable and referenced; new subscriptions
// and seats cannot pick them. Rows are never deleted through the API.
func (s *Service) SetDisabled(ctx context.Context, p *store.Principal, reqID, id string, disabled bool) (any, error) {
	return s.St.Write(ctx, p, reqID, "provider_disable", map[string]any{"id": id, "disabled": disabled}, func(w *store.W) (any, error) {
		if err := w.Human("catalog.providers.disable"); err != nil {
			return nil, err
		}
		tag, err := w.Exec(`UPDATE providers SET disabled_at = CASE WHEN $2 THEN coalesce(disabled_at, now()) END,
			version = version + 1 WHERE id=$1`, id, disabled)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() == 0 {
			return nil, store.ErrNotFound("provider " + id)
		}
		var out json.RawMessage
		if err := w.Row(selectRow+` WHERE t.id=$1`, id).Scan(&out); err != nil {
			return nil, err
		}
		return out, w.Event("provider.disabled", nil, nil, map[string]any{"id": id, "disabled": disabled})
	})
}

// ModelIn is one entry of a hand-kept model list (models.source static, or a manual refresh).
type ModelIn struct {
	Model string `json:"model"`
	Label string `json:"label,omitempty"`
}

// PutModels replaces a provider's model cache with the given list, in order.
func (s *Service) PutModels(ctx context.Context, p *store.Principal, reqID, id string, list []ModelIn) (any, error) {
	return s.St.Write(ctx, p, reqID, "provider_models", map[string]any{"id": id, "models": list}, func(w *store.W) (any, error) {
		if err := w.Human("catalog.providers.put"); err != nil {
			return nil, err
		}
		if err := w.Row(`SELECT id FROM providers WHERE id=$1`, id).Scan(new(string)); err != nil {
			return nil, store.ErrNotFound("provider " + id)
		}
		if _, err := w.Exec(`DELETE FROM provider_models WHERE provider_id=$1`, id); err != nil {
			return nil, err
		}
		if err := putModels(w, id, list, "manual"); err != nil {
			return nil, err
		}
		return map[string]any{"id": id, "models": len(list)}, w.Event("provider.models", nil, nil, map[string]any{"id": id, "count": len(list)})
	})
}
