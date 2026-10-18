package changepolicy

import (
	"context"
	"encoding/json"
	"strings"

	"agenthub/internal/policyrules"
	"agenthub/internal/store"
)

// TicketIn creates or edits a change ticket. Command is the exact command line the change will run; a call can use
// the ticket only when its command and machine match (code invariant).
type TicketIn struct {
	ID      string              `json:"id"`
	Title   string              `json:"title"`
	Machine string              `json:"machine,omitempty"`
	Command string              `json:"command"`
	Fields  map[string]string   `json:"fields,omitempty" jsonschema:"e.g. topology, blast_radius, snapshot, oob_path, site"`
	Lists   map[string][]string `json:"lists,omitempty" jsonschema:"e.g. research (URLs), verification, rollback"`
	Version int64               `json:"version,omitempty" jsonschema:"current version when editing"`
}

// Create files a draft ticket authored by the caller (agents need the data policy change.ticket).
func (s *Service) Create(ctx context.Context, p *store.Principal, reqID string, in TicketIn) (any, error) {
	return s.St.Write(ctx, p, reqID, "ticket_create", in, func(w *store.W) (any, error) {
		if err := w.Can("change.ticket"); err != nil {
			return nil, err
		}
		if !policyrules.ValidID(in.ID) {
			return nil, store.ErrInvalid("ticket id must match [A-Za-z0-9][A-Za-z0-9._-]{2,63}")
		}
		if err := checkIn(in); err != nil {
			return nil, err
		}
		f, l := jsonb(in.Fields), jsonb(in.Lists)
		if _, err := w.Exec(`INSERT INTO change_tickets(id,tenant_id,author,title,machine_id,command,fields,lists,sha256)
			VALUES($1,$2,$3,$4,nullif($5,''),$6,$7,$8,'')`, in.ID, tenantOf(w, in.Machine), p.ID, strings.TrimSpace(in.Title), in.Machine, in.Command, f, l); err != nil {
			return nil, store.Fail(409, "ticket_exists", "ticket %s exists or its machine is unknown", in.ID)
		}
		t, err := refresh(w, in.ID)
		if err != nil {
			return nil, err
		}
		return t, w.Event("ticket.created", nil, nil, map[string]any{"ticket": in.ID, "machine": in.Machine, "sha256": t.SHA256, "title": in.Title})
	})
}

// Update replaces the content of the caller's own ticket. The hash changes, so every earlier review goes stale.
func (s *Service) Update(ctx context.Context, p *store.Principal, reqID string, in TicketIn) (any, error) {
	return s.St.Write(ctx, p, reqID, "ticket_update", in, func(w *store.W) (any, error) {
		if err := checkIn(in); err != nil {
			return nil, err
		}
		tag, err := w.Exec(`UPDATE change_tickets SET title=$3, machine_id=nullif($4,''), command=$5, fields=$6, lists=$7, version=version+1, updated_at=now()
			WHERE id=$1 AND author=$2 AND version=$8`, in.ID, p.ID, strings.TrimSpace(in.Title), in.Machine, in.Command, jsonb(in.Fields), jsonb(in.Lists), in.Version)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() == 0 {
			return nil, store.Fail(409, "ticket_conflict", "ticket %s is not yours or its version moved", in.ID)
		}
		t, err := refresh(w, in.ID)
		if err != nil {
			return nil, err
		}
		return t, w.Event("ticket.updated", nil, nil, map[string]any{"ticket": in.ID, "sha256": t.SHA256})
	})
}

// Submit moves the caller's draft into the review queue.
func (s *Service) Submit(ctx context.Context, p *store.Principal, reqID, id string) (any, error) {
	return s.St.Write(ctx, p, reqID, "ticket_submit", id, func(w *store.W) (any, error) {
		tag, err := w.Exec(`UPDATE change_tickets SET status='pending_review', updated_at=now(), version=version+1 WHERE id=$1 AND author=$2 AND status='draft'`, id, p.ID)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() == 0 {
			return nil, store.Fail(409, "not_submittable", "ticket %s is not your draft", id)
		}
		t, err := refresh(w, id)
		if err != nil {
			return nil, err
		}
		return t, w.Event("ticket.submitted", nil, nil, map[string]any{"ticket": id, "sha256": t.SHA256})
	})
}

func checkIn(in TicketIn) error {
	switch {
	case len(strings.TrimSpace(in.Title)) < 3:
		return store.ErrInvalid("title is required")
	case strings.TrimSpace(in.Command) == "":
		return store.ErrInvalid("command is required: the exact command line this change runs")
	}
	return nil
}

func jsonb(v any) []byte {
	b, _ := json.Marshal(v)
	if string(b) == "null" {
		return []byte("{}")
	}
	return b
}
