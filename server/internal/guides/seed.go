// Package guides stores, edits, publishes, imports and runs guides on top of guidekit and the store.
package guides

import (
	"encoding/json"
	"fmt"
	"regexp"

	"agenthub/internal/guidekit"
	"agenthub/internal/store"
)

// Hooks connect guide runs to machines and the access flow without an import cycle.
type Hooks interface {
	// Exec runs an agent step's op on the run's machine under the caller's grants: outcome is ok or fail.
	Exec(w Ctx, machineID string, s guidekit.Step) (outcome string, result any, err error)
	// Request files (or re-checks) the access a request_access step asks for: approved, pending or denied.
	Request(w Ctx, run Run, s guidekit.Step) (status string, ref any, err error)
}

// Service is the guides domain.
type Service struct {
	St     *store.Store
	HubURL string
	Hooks  Hooks
}

// File is a seed or bundle guide document.
type File struct {
	ID        string          `json:"id"`
	Slug      string          `json:"slug"`
	Title     string          `json:"title"`
	Goal      string          `json:"goal"`
	Tags      []string        `json:"tags"`
	AppliesTo json.RawMessage `json:"applies_to"`
	Channel   string          `json:"channel"`
	Version   struct {
		Semver      string          `json:"semver"`
		ChangeNotes string          `json:"change_notes"`
		RiskIDs     []string        `json:"risk_ids"`
		Steps       json.RawMessage `json:"steps"`
	} `json:"version"`
}

func schemaOf(set store.Settings) []byte { return set["guide_step_schema"] }

// SecretRes turns the secret_patterns prefixes into lint patterns.
func SecretRes(set store.Settings) []*regexp.Regexp {
	out := []*regexp.Regexp{}
	for _, p := range set.Strings("secret_patterns") {
		out = append(out, regexp.MustCompile(regexp.QuoteMeta(p)+`[A-Za-z0-9_\-]{12,}`))
	}
	return out
}

// Seed inserts one Shippin seed guide and its published version once (marked so deletions stick).
func Seed(w *store.W, raw []byte) (int, error) {
	var f File
	if err := json.Unmarshal(raw, &f); err != nil {
		return 0, err
	}
	tag, err := w.Exec(`INSERT INTO preset_marks(name) VALUES($1) ON CONFLICT DO NOTHING`, "guides/"+f.ID+"@"+f.Version.Semver)
	if err != nil || tag.RowsAffected() == 0 {
		return 0, err
	}
	set, err := w.Settings()
	if err != nil {
		return 0, err
	}
	steps, err := guidekit.Validate(schemaOf(set), f.Version.Steps)
	if err != nil {
		return 0, fmt.Errorf("guide %s: %v", f.ID, err)
	}
	if f.Channel == "" {
		f.Channel = "stable"
	}
	if _, err := w.Exec(`INSERT INTO guides(id,slug,title,goal,tags,applies_to,source,channel) VALUES($1,$2,$3,$4,$5,$6,'shippin',$7)
		ON CONFLICT (id) DO NOTHING`, f.ID, f.Slug, f.Title, f.Goal, f.Tags, []byte(f.AppliesTo), f.Channel); err != nil {
		return 0, err
	}
	vid, err := insertVersion(w, f, steps, "shippin", "published")
	if err != nil {
		return 0, err
	}
	_, err = w.Exec(`UPDATE guides SET current_version_id=$2 WHERE id=$1`, f.ID, vid)
	return 1, err
}

func insertVersion(w *store.W, f File, steps []guidekit.Step, by, status string) (string, error) {
	b := guidekit.Bundle{GuideID: f.ID, Semver: f.Version.Semver, Title: f.Title, Goal: f.Goal, Tags: f.Tags, AppliesTo: f.AppliesTo,
		ChangeNotes: f.Version.ChangeNotes, RiskIDs: f.Version.RiskIDs, Steps: f.Version.Steps}
	if f.Version.RiskIDs == nil {
		f.Version.RiskIDs = []string{}
	}
	var id string
	err := w.Row(`INSERT INTO guide_versions(guide_id,semver,steps,change_notes,risk_ids,authored_by,content_sha256,status,published_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,CASE WHEN $8='published' THEN now() END) RETURNING id`,
		f.ID, f.Version.Semver, []byte(f.Version.Steps), f.Version.ChangeNotes, f.Version.RiskIDs, by, guidekit.ContentSHA(b), status).Scan(&id)
	if err != nil {
		return "", store.Fail(409, "version_conflict", "guide %s %s: %v", f.ID, f.Version.Semver, err)
	}
	_ = steps
	return id, nil
}
