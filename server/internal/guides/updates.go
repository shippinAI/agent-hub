package guides

import (
	"context"
	"encoding/json"
	"time"

	"agenthub/internal/guidekit"
	"agenthub/internal/store"
)

// Update is one queued Shippin update.
type Update struct {
	ID          string    `json:"id"`
	GuideID     string    `json:"guide_id"`
	From        *string   `json:"from_semver"`
	To          string    `json:"to_semver"`
	ChangeNotes string    `json:"change_notes"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// Updates lists queued updates.
func (s *Service) Updates(ctx context.Context) ([]Update, error) {
	rows, err := s.St.DB.Query(ctx, `SELECT id::text,guide_id,from_semver,to_semver,change_notes,status,created_at FROM guide_updates ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Update{}
	for rows.Next() {
		var u Update
		if err := rows.Scan(&u.ID, &u.GuideID, &u.From, &u.To, &u.ChangeNotes, &u.Status, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// Decide accepts (publish upstream), skips, or merges (upstream steps become a draft on the tenant override).
func (s *Service) Decide(ctx context.Context, p *store.Principal, reqID, id, decision string) (any, error) {
	return s.St.Write(ctx, p, reqID, "guide_update_decide", map[string]string{"id": id, "d": decision}, func(w *store.W) (any, error) {
		if err := w.Human("guide.publish"); err != nil {
			return nil, err
		}
		var raw []byte
		if err := w.Row(`SELECT payload FROM guide_updates WHERE id::text=$1 AND status='pending' FOR UPDATE`, id).Scan(&raw); err != nil {
			return nil, store.Fail(409, "not_pending", "update is not pending")
		}
		var b guidekit.Bundle
		_ = json.Unmarshal(raw, &b)
		f := File{ID: b.GuideID, Title: b.Title, Goal: b.Goal, Tags: b.Tags, AppliesTo: b.AppliesTo}
		f.Version.Semver, f.Version.ChangeNotes, f.Version.RiskIDs, f.Version.Steps = b.Semver, b.ChangeNotes, b.RiskIDs, b.Steps
		var out any = map[string]any{"id": id}
		switch decision {
		case "accept":
			r, err := apply(w, f, b, "accepted")
			if err != nil {
				return nil, err
			}
			out = r
		case "merge":
			var oid string
			if err := w.Row(`SELECT id FROM guides WHERE overrides_guide_id=$1`, b.GuideID).Scan(&oid); err != nil {
				return nil, store.Fail(409, "no_override", "merge needs a tenant override; accept instead")
			}
			f.ID = oid
			f.Version.ChangeNotes = "Merged from Shippin " + b.Semver + ": " + b.ChangeNotes
			vid, err := insertVersion(w, f, nil, w.P.ID, "draft")
			if err != nil {
				return nil, err
			}
			out = map[string]any{"id": id, "draft_version_id": vid, "guide_id": oid}
		case "skip":
		default:
			return nil, store.ErrInvalid("decision must be accept, merge or skip")
		}
		st := map[string]string{"accept": "accepted", "merge": "merged", "skip": "skipped"}[decision]
		if _, err := w.Exec(`UPDATE guide_updates SET status=$2, decided_by=$3, decided_at=now() WHERE id::text=$1`, id, st, w.P.ID); err != nil {
			return nil, err
		}
		return out, w.Event("guide.update_"+st, nil, nil, map[string]any{"guide": b.GuideID, "to": b.Semver})
	})
}
