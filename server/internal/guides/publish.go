package guides

import (
	"context"

	"agenthub/internal/guidekit"
	"agenthub/internal/store"
)

// Publish makes a draft the guide's current version after a clean lint. Published versions are immutable.
func (s *Service) Publish(ctx context.Context, p *store.Principal, reqID, versionID string) (any, error) {
	return s.St.Write(ctx, p, reqID, "guide_publish", versionID, func(w *store.W) (any, error) {
		if err := w.Human("guide.publish"); err != nil {
			return nil, err
		}
		var gid, status string
		var raw, applies []byte
		err := w.Row(`SELECT v.guide_id,v.status,v.steps,g.applies_to FROM guide_versions v JOIN guides g ON g.id=v.guide_id
			WHERE v.id::text=$1 FOR UPDATE OF v`, versionID).Scan(&gid, &status, &raw, &applies)
		if err != nil {
			return nil, store.ErrNotFound("guide version " + versionID)
		}
		if status != "draft" {
			return nil, store.Fail(409, "not_draft", "version is %s", status)
		}
		set, _ := w.Settings()
		steps, err := guidekit.Validate(schemaOf(set), raw)
		if err != nil {
			return nil, store.Fail(422, "invalid_steps", "%v", err)
		}
		if lint := guidekit.Lint(steps, oses(applies), SecretRes(set)); guidekit.Blocking(lint) {
			return nil, &store.Error{Status: 422, Code: "lint_blocked", Message: "fix the blocking lint findings first", Current: lint}
		}
		if _, err := w.Exec(`UPDATE guide_versions SET status='published', published_at=now(), signed_by=$2 WHERE id::text=$1`, versionID, w.P.ID); err != nil {
			return nil, err
		}
		if _, err := w.Exec(`UPDATE guides SET current_version_id=$2::uuid, version=version+1 WHERE id=$1`, gid, versionID); err != nil {
			return nil, err
		}
		return map[string]any{"guide_id": gid, "version_id": versionID}, w.Event("guide.published", nil, nil, map[string]any{"guide": gid, "version": versionID})
	})
}

// Retire retires a published version; a retired current version leaves the guide without one.
func (s *Service) Retire(ctx context.Context, p *store.Principal, reqID, versionID string) (any, error) {
	return s.St.Write(ctx, p, reqID, "guide_retire", versionID, func(w *store.W) (any, error) {
		if err := w.Human("guide.publish"); err != nil {
			return nil, err
		}
		var gid string
		if err := w.Row(`UPDATE guide_versions SET status='retired' WHERE id::text=$1 AND status='published' RETURNING guide_id`, versionID).Scan(&gid); err != nil {
			return nil, store.Fail(409, "not_published", "only a published version can be retired")
		}
		if _, err := w.Exec(`UPDATE guides SET current_version_id=NULL, version=version+1 WHERE id=$1 AND current_version_id::text=$2`, gid, versionID); err != nil {
			return nil, err
		}
		return map[string]any{"guide_id": gid, "retired": versionID}, w.Event("guide.retired", nil, nil, map[string]any{"guide": gid, "version": versionID})
	})
}
