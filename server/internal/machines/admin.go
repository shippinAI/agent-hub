package machines

import (
	"context"

	"agenthub/internal/store"
)

// Patch edits label and shell profiles (optimistic on version).
func (s *Service) Patch(ctx context.Context, p *store.Principal, reqID, id string, label string, profiles []string, version int64) (any, error) {
	return s.St.Write(ctx, p, reqID, "machine_patch", map[string]any{"id": id, "label": label, "p": profiles, "v": version}, func(w *store.W) (any, error) {
		if err := w.Human("machine.edit"); err != nil {
			return nil, err
		}
		tag, err := w.Exec(`UPDATE machines SET label=coalesce(nullif($2,''),label), shell_profile_ids=coalesce($3,shell_profile_ids), version=version+1
			WHERE id=$1 AND version=$4`, id, label, profiles, version)
		if err != nil {
			return nil, store.ErrInvalid("%v", err)
		}
		if tag.RowsAffected() == 0 {
			return nil, store.ErrVersion(nil)
		}
		return map[string]any{"id": id}, w.Event("machine.updated", nil, nil, map[string]any{"machine": id})
	})
}

// SetPaused pauses (refuses every call) or resumes a machine. Resuming also clears a GUI stop.
func (s *Service) SetPaused(ctx context.Context, p *store.Principal, reqID, id string, paused bool) (any, error) {
	act := map[bool]string{true: "paused", false: "resumed"}[paused]
	return s.St.Write(ctx, p, reqID, "machine_"+act, id, func(w *store.W) (any, error) {
		if err := w.Human("machine.pause"); err != nil {
			return nil, err
		}
		tag, err := w.Exec(`UPDATE machines SET disabled=$2, gui_paused=CASE WHEN $2 THEN gui_paused ELSE false END, version=version+1 WHERE id=$1`, id, paused)
		if err != nil || tag.RowsAffected() == 0 {
			return nil, store.ErrNotFound("machine " + id)
		}
		return map[string]any{"id": id, "disabled": paused}, w.Event("machine."+act, nil, nil, map[string]any{"machine": id})
	})
}

// Remove revokes the bridge token and disables the machine principal; grants are ended by the caller in the
// same request (access.EndAllForMachine) so the bridge is told to stop.
func (s *Service) Remove(ctx context.Context, p *store.Principal, reqID, id string) (any, error) {
	return s.St.Write(ctx, p, reqID, "machine_remove", id, func(w *store.W) (any, error) {
		if err := w.Human("machine.remove"); err != nil {
			return nil, err
		}
		var pid string
		if err := w.Row(`UPDATE machines SET disabled=true, sign_key_ct=NULL, connected=false, version=version+1 WHERE id=$1 RETURNING principal_id`, id).Scan(&pid); err != nil {
			return nil, store.ErrNotFound("machine " + id)
		}
		if _, err := w.Exec(`UPDATE tokens SET revoked_at=now() WHERE principal_id=$1 AND revoked_at IS NULL`, pid); err != nil {
			return nil, err
		}
		if _, err := w.Exec(`UPDATE principals SET enabled=false WHERE id=$1`, pid); err != nil {
			return nil, err
		}
		return map[string]any{"id": id, "removed": true}, w.Event("machine.removed", nil, nil, map[string]any{"machine": id})
	})
}
