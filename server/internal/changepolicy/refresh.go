package changepolicy

import (
	"encoding/json"

	"agenthub/internal/store"
)

// Ticket is the stored ticket with its derived review state.
type Ticket struct {
	ID        string              `json:"id"`
	Tenant    string              `json:"tenant_id"`
	Author    string              `json:"author"`
	Title     string              `json:"title"`
	Machine   *string             `json:"machine"`
	Command   string              `json:"command"`
	Fields    map[string]string   `json:"fields"`
	Lists     map[string][]string `json:"lists"`
	SHA256    string              `json:"sha256"`
	Status    string              `json:"status"`
	Version   int64               `json:"version"`
	UpdatedAt string              `json:"updated_at"`
}

const ticketCols = `id, tenant_id, author, title, machine_id, command, fields, lists, sha256, status, version, to_char(updated_at,'YYYY-MM-DD"T"HH24:MI:SSOF')`

func scanTicket(r interface{ Scan(...any) error }) (Ticket, error) {
	var t Ticket
	var f, l []byte
	err := r.Scan(&t.ID, &t.Tenant, &t.Author, &t.Title, &t.Machine, &t.Command, &f, &l, &t.SHA256, &t.Status, &t.Version, &t.UpdatedAt)
	_ = json.Unmarshal(f, &t.Fields)
	_ = json.Unmarshal(l, &t.Lists)
	return t, err
}

// refresh recomputes the content hash (title, machine, command, fields, lists and evidence hashes) and the status
// from the reviews on that hash: any BLOCK -> blocked, else any PASS -> passed, else FINDINGS -> findings.
func refresh(w q, id string) (Ticket, error) {
	t, err := scanTicket(w.Row(`SELECT `+ticketCols+` FROM change_tickets WHERE id=$1`, id))
	if err != nil {
		return t, store.ErrNotFound("ticket " + id)
	}
	var ev []string
	rows, err := w.Query(`SELECT path || '=' || sha256 FROM ticket_evidence WHERE ticket_id=$1 ORDER BY id`, id)
	if err != nil {
		return t, err
	}
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		ev = append(ev, s)
	}
	rows.Close()
	sha := shaJSON([]any{t.Title, t.Machine, t.Command, t.Fields, t.Lists, ev})
	st := t.Status
	if st != "draft" && st != "ran" {
		var pass, block, finds int
		_ = w.Row(`SELECT count(*) FILTER (WHERE verdict='PASS'), count(*) FILTER (WHERE verdict='BLOCK'), count(*) FILTER (WHERE verdict='FINDINGS')
			FROM (SELECT DISTINCT ON (reviewer) verdict FROM ticket_reviews WHERE ticket_id=$1 AND ticket_sha256=$2 ORDER BY reviewer, id DESC) v`, id, sha).Scan(&pass, &block, &finds)
		st = map[bool]string{true: "blocked"}[block > 0]
		switch {
		case st != "":
		case pass > 0:
			st = "passed"
		case finds > 0:
			st = "findings"
		default:
			st = "pending_review"
		}
	}
	if sha != t.SHA256 || st != t.Status {
		if err := w.Row(`UPDATE change_tickets SET sha256=$2, status=$3 WHERE id=$1 RETURNING id`, id, sha, st).Scan(new(string)); err != nil {
			return t, err
		}
		t.SHA256, t.Status = sha, st
	}
	return t, nil
}
