package changepolicy

import (
	"context"
	"encoding/json"
	"time"

	"agenthub/internal/store"
)

// EvidenceRow is one attached artifact.
type EvidenceRow struct {
	Kind       string    `json:"kind"`
	Path       string    `json:"path"`
	SHA256     string    `json:"sha256"`
	Excerpt    string    `json:"excerpt"`
	CapturedAt time.Time `json:"captured_at"`
	AddedBy    string    `json:"added_by"`
}

// ReviewRow is one verdict; Current says whether it was given on the ticket's current hash.
type ReviewRow struct {
	Reviewer  string          `json:"reviewer"`
	Verdict   string          `json:"verdict"`
	Summary   string          `json:"summary"`
	Findings  json.RawMessage `json:"findings"`
	SHA256    string          `json:"ticket_sha256"`
	Current   bool            `json:"current"`
	CreatedAt time.Time       `json:"created_at"`
}

// Detail is a ticket with its evidence and reviews.
type Detail struct {
	Ticket   Ticket        `json:"ticket"`
	Evidence []EvidenceRow `json:"evidence"`
	Reviews  []ReviewRow   `json:"reviews"`
}

// Get returns one ticket with evidence and reviews.
func (s *Service) Get(ctx context.Context, id string) (Detail, error) {
	var d Detail
	t, err := scanTicket(s.St.DB.QueryRow(ctx, `SELECT `+ticketCols+` FROM change_tickets WHERE id=$1`, id))
	if err != nil {
		return d, store.ErrNotFound("ticket " + id)
	}
	d.Ticket, d.Evidence, d.Reviews = t, []EvidenceRow{}, []ReviewRow{}
	rows, err := s.St.DB.Query(ctx, `SELECT kind, path, sha256, excerpt, captured_at, added_by FROM ticket_evidence WHERE ticket_id=$1 ORDER BY id`, id)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var e EvidenceRow
		_ = rows.Scan(&e.Kind, &e.Path, &e.SHA256, &e.Excerpt, &e.CapturedAt, &e.AddedBy)
		d.Evidence = append(d.Evidence, e)
	}
	rows.Close()
	rows, err = s.St.DB.Query(ctx, `SELECT reviewer, verdict, summary, findings, ticket_sha256, created_at FROM ticket_reviews WHERE ticket_id=$1 ORDER BY id`, id)
	if err != nil {
		return d, err
	}
	defer rows.Close()
	for rows.Next() {
		var r ReviewRow
		_ = rows.Scan(&r.Reviewer, &r.Verdict, &r.Summary, &r.Findings, &r.SHA256, &r.CreatedAt)
		r.Current = r.SHA256 == t.SHA256
		d.Reviews = append(d.Reviews, r)
	}
	return d, rows.Err()
}

// List returns tickets (optionally one status), newest first. queueFor, when set, keeps only tickets in review
// that this principal did not author (the reviewer's queue).
func (s *Service) List(ctx context.Context, status, queueFor string) ([]Ticket, error) {
	rows, err := s.St.DB.Query(ctx, `SELECT `+ticketCols+` FROM change_tickets WHERE ($1='' OR status=$1)
		AND ($2='' OR (author<>$2 AND status IN ('pending_review','findings','blocked'))) ORDER BY updated_at DESC LIMIT 200`, status, queueFor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Ticket{}
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
