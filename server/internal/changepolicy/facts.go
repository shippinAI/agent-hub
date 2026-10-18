package changepolicy

import (
	"fmt"

	"agenthub/internal/policyrules"
)

// dbFacts reads tickets, evidence and current-hash reviews inside the decision transaction. Ticket binds the
// ticket to the call (code invariants): the caller is its author, and the command and machine match exactly.
type dbFacts struct {
	w       q
	caller  string
	command string
	machine string
	use     bool // false = preview (review_ticket / UI): skip the caller and command binding
}

func (f dbFacts) Ticket(id string) (*policyrules.Ticket, error) {
	t, err := scanTicket(f.w.Row(`SELECT `+ticketCols+` FROM change_tickets WHERE id=$1`, id))
	if err != nil {
		return nil, fmt.Errorf("ticket %s not found", id)
	}
	if f.use {
		switch {
		case t.Author != f.caller:
			return nil, fmt.Errorf("ticket %s belongs to %s; only its author can run it", id, t.Author)
		case t.Command != f.command:
			return nil, fmt.Errorf("ticket %s covers a different command; the call must run exactly the ticket's command", id)
		case t.Machine != nil && *t.Machine != f.machine:
			return nil, fmt.Errorf("ticket %s is for machine %s", id, *t.Machine)
		case t.Status == "draft":
			return nil, fmt.Errorf("ticket %s is still a draft; submit it for review", id)
		}
	}
	return &policyrules.Ticket{ID: t.ID, Author: t.Author, SHA256: t.SHA256, Fields: t.Fields, Lists: t.Lists}, nil
}

func (f dbFacts) Evidence(t *policyrules.Ticket, path string) (policyrules.Evidence, bool) {
	e := policyrules.Evidence{Path: path}
	err := f.w.Row(`SELECT excerpt, captured_at FROM ticket_evidence WHERE ticket_id=$1 AND path=$2 ORDER BY captured_at DESC LIMIT 1`, t.ID, path).
		Scan(&e.Text, &e.CapturedAt)
	return e, err == nil
}

func (f dbFacts) Reviews(t *policyrules.Ticket) []policyrules.Review {
	rows, err := f.w.Query(`SELECT r.reviewer, r.verdict, p.is_human, p.roles FROM ticket_reviews r JOIN principals p ON p.id=r.reviewer
		WHERE r.ticket_id=$1 AND r.ticket_sha256=$2 AND p.enabled ORDER BY r.id`, t.ID, t.SHA256)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []policyrules.Review
	for rows.Next() {
		var r policyrules.Review
		if rows.Scan(&r.Reviewer, &r.Verdict, &r.Human, &r.Roles) == nil {
			out = append(out, r)
		}
	}
	return out
}
