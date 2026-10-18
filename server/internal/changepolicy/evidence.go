package changepolicy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"agenthub/internal/store"
)

// EvidenceIn attaches one artifact. Text is stored as an excerpt (max 20000 chars) with its sha256; it must not hold
// secrets (the hub redacts nothing here, so callers send command output they would show a reviewer).
type EvidenceIn struct {
	Ticket string `json:"ticket"`
	Kind   string `json:"kind" jsonschema:"snapshot | topology | research | output | doc"`
	Path   string `json:"path" jsonschema:"absolute path the ticket fields refer to (e.g. snapshot: /var/backups/x.txt)"`
	Text   string `json:"text"`
}

// AddEvidence appends evidence to the caller's own ticket; the ticket hash changes, so reviews go stale.
func (s *Service) AddEvidence(ctx context.Context, p *store.Principal, reqID string, in EvidenceIn) (any, error) {
	return s.St.Write(ctx, p, reqID, "ticket_evidence", in, func(w *store.W) (any, error) {
		var author string
		if err := w.Row(`SELECT author FROM change_tickets WHERE id=$1 FOR UPDATE`, in.Ticket).Scan(&author); err != nil {
			return nil, store.ErrNotFound("ticket " + in.Ticket)
		}
		if author != p.ID {
			return nil, store.Fail(403, "not_author", "only the author adds evidence to %s", in.Ticket)
		}
		if !strings.HasPrefix(in.Path, "/") || len(in.Text) > 20000 || strings.TrimSpace(in.Kind) == "" {
			return nil, store.ErrInvalid("evidence needs kind, an absolute path and text up to 20000 chars")
		}
		h := sha256.Sum256([]byte(in.Text))
		sum := hex.EncodeToString(h[:])
		if _, err := w.Exec(`INSERT INTO ticket_evidence(ticket_id,kind,path,sha256,excerpt,added_by) VALUES($1,$2,$3,$4,$5,$6)`,
			in.Ticket, in.Kind, in.Path, sum, in.Text, p.ID); err != nil {
			return nil, err
		}
		t, err := refresh(w, in.Ticket)
		if err != nil {
			return nil, err
		}
		return t, w.Event("ticket.evidence", nil, nil, map[string]any{"ticket": in.Ticket, "kind": in.Kind, "path": in.Path, "sha256": sum, "ticket_sha256": t.SHA256})
	})
}
