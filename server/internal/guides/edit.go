package guides

import (
	"context"
	"encoding/json"
	"fmt"

	"agenthub/internal/guidekit"
	"agenthub/internal/store"
)

// Draft is an edit request: a guide (new or existing) and the steps of a draft version.
type Draft struct {
	GuideID     string          `json:"guide_id"`
	Title       string          `json:"title"`
	Goal        string          `json:"goal"`
	Tags        []string        `json:"tags"`
	AppliesTo   json.RawMessage `json:"applies_to"`
	Semver      string          `json:"semver"`
	ChangeNotes string          `json:"change_notes"`
	RiskIDs     []string        `json:"risk_ids"`
	Steps       json.RawMessage `json:"steps"`
}

// SaveDraft creates or replaces a draft version. Editing a Shippin guide creates (or reuses) the tenant
// override "local-<id>"; Shippin rows themselves are never edited. Returns the version id and lint findings.
func (s *Service) SaveDraft(ctx context.Context, p *store.Principal, reqID string, d Draft) (any, error) {
	return s.St.Write(ctx, p, reqID, "guide_draft", d, func(w *store.W) (any, error) {
		if err := w.Human("guide.edit"); err != nil {
			return nil, err
		}
		if _, err := guidekit.Semver(d.Semver); err != nil {
			return nil, store.ErrInvalid("%v", err)
		}
		set, err := w.Settings()
		if err != nil {
			return nil, err
		}
		steps, err := guidekit.Validate(schemaOf(set), d.Steps)
		if err != nil {
			return nil, store.Fail(422, "invalid_steps", "%v", err)
		}
		gid, err := s.target(w, &d)
		if err != nil {
			return nil, err
		}
		if _, err := w.Exec(`DELETE FROM guide_versions WHERE guide_id=$1 AND semver=$2 AND status='draft'`, gid, d.Semver); err != nil {
			return nil, err
		}
		f := File{ID: gid, Title: d.Title, Goal: d.Goal, Tags: d.Tags, AppliesTo: d.AppliesTo}
		f.Version.Semver, f.Version.ChangeNotes, f.Version.RiskIDs, f.Version.Steps = d.Semver, d.ChangeNotes, d.RiskIDs, d.Steps
		vid, err := insertVersion(w, f, steps, w.P.ID, "draft")
		if err != nil {
			return nil, err
		}
		lint := guidekit.Lint(steps, oses(d.AppliesTo), SecretRes(set))
		return map[string]any{"guide_id": gid, "version_id": vid, "lint": lint, "blocking": guidekit.Blocking(lint)},
			w.Event("guide.draft_saved", nil, nil, map[string]any{"guide": gid, "semver": d.Semver})
	})
}

func (s *Service) target(w *store.W, d *Draft) (string, error) {
	if d.Tags == nil {
		d.Tags = []string{}
	}
	if len(d.AppliesTo) == 0 {
		d.AppliesTo = json.RawMessage(`{}`)
	}
	var src string
	var title, goal string
	err := w.Row(`SELECT source,title,goal FROM guides WHERE id=$1`, d.GuideID).Scan(&src, &title, &goal)
	if err != nil {
		if d.GuideID == "" || d.Title == "" {
			return "", store.ErrInvalid("a new guide needs guide_id and title")
		}
		_, err := w.Exec(`INSERT INTO guides(id,slug,title,goal,tags,applies_to,source,tenant_id) VALUES($1,$1,$2,$3,$4,$5,'tenant','local')`,
			d.GuideID, d.Title, d.Goal, d.Tags, []byte(d.AppliesTo))
		return d.GuideID, err
	}
	if d.Title == "" {
		d.Title, d.Goal = title, goal
	}
	if src != "shippin" {
		_, err := w.Exec(`UPDATE guides SET title=$2,goal=$3,tags=$4,applies_to=$5,version=version+1 WHERE id=$1`, d.GuideID, d.Title, d.Goal, d.Tags, []byte(d.AppliesTo))
		return d.GuideID, err
	}
	oid := "local-" + d.GuideID
	_, err = w.Exec(`INSERT INTO guides(id,slug,title,goal,tags,applies_to,source,overrides_guide_id,tenant_id)
		VALUES($1,$1,$2,$3,$4,$5,'tenant',$6,'local') ON CONFLICT (id) DO NOTHING`, oid, d.Title, d.Goal, d.Tags, []byte(d.AppliesTo), d.GuideID)
	return oid, err
}

func oses(appliesTo json.RawMessage) []string {
	var a struct {
		OS []string `json:"os"`
	}
	_ = json.Unmarshal(appliesTo, &a)
	if len(a.OS) == 0 {
		return []string{"linux", "macos", "windows"}
	}
	return a.OS
}

// SetEnabled turns a guide (or override) on or off.
func (s *Service) SetEnabled(ctx context.Context, p *store.Principal, reqID, id string, on bool) (any, error) {
	return s.St.Write(ctx, p, reqID, "guide_enable", fmt.Sprint(id, on), func(w *store.W) (any, error) {
		if err := w.Human("guide.edit"); err != nil {
			return nil, err
		}
		tag, err := w.Exec(`UPDATE guides SET enabled=$2, version=version+1 WHERE id=$1`, id, on)
		if err != nil || tag.RowsAffected() == 0 {
			return nil, store.ErrNotFound("guide " + id)
		}
		return map[string]any{"id": id, "enabled": on}, w.Event("guide.updated", nil, nil, map[string]any{"guide": id, "enabled": on})
	})
}
