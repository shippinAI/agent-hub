package changepolicy

import (
	"context"
	"strings"

	"agenthub/internal/store"
)

// ReviewIn is a reviewer's verdict on the ticket hash they looked at.
type ReviewIn struct {
	Ticket   string   `json:"ticket"`
	Verdict  string   `json:"verdict" jsonschema:"PASS | BLOCK | FINDINGS"`
	Summary  string   `json:"summary"`
	Findings []string `json:"findings,omitempty"`
	SHA256   string   `json:"ticket_sha256" jsonschema:"the sha256 you reviewed (from review_ticket); a stale hash is refused"`
}

// Review records a verdict. Code invariants: never on your own ticket, only on the current hash, only when the
// ticket is in review. Who may review at all is the data policy change.review (preset: role:reviewer and humans).
func (s *Service) Review(ctx context.Context, p *store.Principal, reqID string, in ReviewIn) (any, error) {
	return s.St.Write(ctx, p, reqID, "ticket_review", in, func(w *store.W) (any, error) {
		in.Verdict = strings.ToUpper(strings.TrimSpace(in.Verdict))
		if in.Verdict != "PASS" && in.Verdict != "BLOCK" && in.Verdict != "FINDINGS" {
			return nil, store.ErrInvalid("verdict must be PASS, BLOCK or FINDINGS")
		}
		if len(strings.TrimSpace(in.Summary)) < 3 {
			return nil, store.ErrInvalid("summary is required")
		}
		var author, sha, status string
		if err := w.Row(`SELECT author, sha256, status FROM change_tickets WHERE id=$1 FOR UPDATE`, in.Ticket).Scan(&author, &sha, &status); err != nil {
			return nil, store.ErrNotFound("ticket " + in.Ticket)
		}
		if author == p.ID {
			return nil, store.Fail(403, "self_review", "you can't review your own ticket %s", in.Ticket)
		}
		if err := w.Can("change.review"); err != nil {
			return nil, err
		}
		if status == "draft" || status == "ran" {
			return nil, store.Fail(409, "not_in_review", "ticket %s is %s", in.Ticket, status)
		}
		if in.SHA256 != sha {
			return nil, store.Fail(409, "stale_review", "ticket %s changed since you looked; re-read it (current sha256 %s)", in.Ticket, sha[:12])
		}
		if _, err := w.Exec(`INSERT INTO ticket_reviews(ticket_id,reviewer,verdict,summary,findings,ticket_sha256) VALUES($1,$2,$3,$4,$5,$6)`,
			in.Ticket, p.ID, in.Verdict, strings.TrimSpace(in.Summary), jsonList(in.Findings), sha); err != nil {
			return nil, err
		}
		t, err := refresh(w, in.Ticket)
		if err != nil {
			return nil, err
		}
		return t, w.Event("ticket.reviewed", nil, nil, map[string]any{"ticket": in.Ticket, "verdict": in.Verdict, "ticket_sha256": sha,
			"findings": len(in.Findings), "status": t.Status})
	})
}

func jsonList(v []string) []byte {
	if v == nil {
		v = []string{}
	}
	return jsonb(v)
}
