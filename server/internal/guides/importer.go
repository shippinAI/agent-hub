package guides

import (
	"context"
	"encoding/json"

	"agenthub/internal/guidekit"
	"agenthub/internal/store"
)

// Import takes a signed Shippin bundle. A bad or untrusted signature is refused and audited; a new guide is
// added at once; a newer version of a known guide is queued as an update (auto_minor applies minor and patch
// bumps of guides without a tenant override straight away).
func (s *Service) Import(ctx context.Context, p *store.Principal, reqID string, b guidekit.Bundle) (any, error) {
	var refusal *store.Error
	out, err := s.St.Write(ctx, p, reqID, "guide_import", b, func(w *store.W) (any, error) {
		if err := w.Human("guide.publish"); err != nil {
			return nil, err
		}
		set, _ := w.Settings()
		var keys []guidekit.TrustedKey
		_ = json.Unmarshal(set["guide_signing_keys"], &keys)
		if err := guidekit.Verify(b, keys); err != nil {
			refusal = store.Fail(422, "bad_signature", "guide bundle refused: %v", err)
			return nil, w.Event("guide.import_rejected", nil, nil, map[string]any{"guide": b.GuideID, "semver": b.Semver, "key_id": b.KeyID, "why": err.Error()})
		}
		if _, err := guidekit.Validate(schemaOf(set), b.Steps); err != nil {
			return nil, store.Fail(422, "invalid_steps", "%v", err)
		}
		f := File{ID: b.GuideID, Slug: b.GuideID, Title: b.Title, Goal: b.Goal, Tags: b.Tags, AppliesTo: b.AppliesTo}
		f.Version.Semver, f.Version.ChangeNotes, f.Version.RiskIDs, f.Version.Steps = b.Semver, b.ChangeNotes, b.RiskIDs, b.Steps
		var cur *string
		var override bool
		err := w.Row(`SELECT v.semver, EXISTS(SELECT 1 FROM guides o WHERE o.overrides_guide_id=g.id) FROM guides g
			LEFT JOIN guide_versions v ON v.id=g.current_version_id WHERE g.id=$1`, b.GuideID).Scan(&cur, &override)
		if err != nil {
			if f.Tags == nil {
				f.Tags = []string{}
			}
			if len(f.AppliesTo) == 0 {
				f.AppliesTo = json.RawMessage(`{}`)
			}
			if _, err := w.Exec(`INSERT INTO guides(id,slug,title,goal,tags,applies_to,source) VALUES($1,$1,$2,$3,$4,$5,'shippin')`,
				f.ID, f.Title, f.Goal, f.Tags, []byte(f.AppliesTo)); err != nil {
				return nil, err
			}
			return apply(w, f, b, "new")
		}
		bump := guidekit.Bump(deref(cur), b.Semver)
		if cur != nil && bump == "" {
			return nil, store.Fail(409, "not_newer", "%s is not newer than %s", b.Semver, *cur)
		}
		if set.String("guide_auto_update", "notify") == "auto_minor" && !override && (bump == "minor" || bump == "patch") {
			return apply(w, f, b, bump)
		}
		payload, _ := json.Marshal(b)
		var uid string
		if err := w.Row(`INSERT INTO guide_updates(guide_id,from_semver,to_semver,bundle_sha256,change_notes,payload) VALUES($1,$2,$3,$4,$5,$6) RETURNING id::text`,
			b.GuideID, cur, b.Semver, guidekit.ContentSHA(b), b.ChangeNotes, payload).Scan(&uid); err != nil {
			return nil, err
		}
		return map[string]any{"guide_id": b.GuideID, "queued": uid, "bump": bump},
			w.Event("guide.update_available", nil, nil, map[string]any{"guide": b.GuideID, "to": b.Semver, "update": uid})
	})
	if refusal != nil && err == nil {
		return nil, refusal
	}
	return out, err
}

func apply(w *store.W, f File, b guidekit.Bundle, how string) (any, error) {
	vid, err := insertVersion(w, f, nil, "shippin", "published")
	if err != nil {
		return nil, err
	}
	if _, err := w.Exec(`UPDATE guide_versions SET signed_by=$2, signature=$3 WHERE id::text=$1`, vid, b.KeyID, b.Signature); err != nil {
		return nil, err
	}
	if _, err := w.Exec(`UPDATE guides SET current_version_id=$2::uuid, title=$3, goal=$4, version=version+1 WHERE id=$1`, f.ID, vid, f.Title, f.Goal); err != nil {
		return nil, err
	}
	return map[string]any{"guide_id": f.ID, "applied": vid, "how": how}, w.Event("guide.imported", nil, nil, map[string]any{"guide": f.ID, "semver": b.Semver, "how": how})
}
