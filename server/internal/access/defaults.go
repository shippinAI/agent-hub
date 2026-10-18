package access

import (
	"context"

	"agenthub/internal/store"
)

// SetDefaultPreset changes which preset fills access cards. A preset needing the unsandboxed policy (full
// access) needs typed confirmation of its id and a live tenant-wide policy.
func (s *Service) SetDefaultPreset(ctx context.Context, p *store.Principal, reqID, id, confirm string) (any, error) {
	return s.St.Write(ctx, p, reqID, "preset_default", map[string]string{"id": id, "c": confirm}, func(w *store.W) (any, error) {
		if err := w.Human("preset.default"); err != nil {
			return nil, err
		}
		var full bool
		var needs *string
		if err := w.Row(`SELECT full_access, requires_policy FROM grant_presets WHERE id=$1`, id).Scan(&full, &needs); err != nil {
			return nil, store.ErrNotFound("grant preset " + id)
		}
		if full || (needs != nil && *needs == "unsandboxed") {
			if confirm != id {
				return nil, store.Fail(422, "confirm_required", "type the preset id %q to confirm", id)
			}
			var live bool
			_ = w.Row(`SELECT EXISTS (SELECT 1 FROM sandbox_policies WHERE machine_id IS NULL AND unsandboxed_allowed AND (expires_at IS NULL OR expires_at > now()))`).Scan(&live)
			if !live {
				return nil, store.Fail(403, "policy_denied", "enable the tenant-wide unsandboxed policy first")
			}
		}
		var old string
		_ = w.Row(`SELECT coalesce(value #>> '{}','') FROM settings WHERE key='grant_default_preset'`).Scan(&old)
		if _, err := w.Exec(`UPDATE grant_presets SET is_default=false WHERE is_default AND id<>$1`, id); err != nil {
			return nil, err
		}
		if _, err := w.Exec(`UPDATE grant_presets SET is_default=true WHERE id=$1`, id); err != nil {
			return nil, err
		}
		if _, err := w.Exec(`INSERT INTO settings(key,value,description) VALUES('grant_default_preset',to_jsonb($1::text),'Preset that fills access cards by default')
			ON CONFLICT (key) DO UPDATE SET value=to_jsonb($1::text), version=settings.version+1`, id); err != nil {
			return nil, err
		}
		return map[string]any{"default_preset": id}, w.Event("preset.default_changed", nil, nil, map[string]any{"from": old, "to": id, "full_access": full})
	})
}
