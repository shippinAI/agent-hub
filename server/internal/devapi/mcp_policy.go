package devapi

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"agenthub/internal/changepolicy"
	"agenthub/internal/mcptools"
	"agenthub/internal/store"
)

type ticketIDIn struct {
	Ticket string `json:"ticket"`
}

type queueIn struct {
	Status string `json:"status,omitempty" jsonschema:"filter: pending_review | findings | blocked"`
}

// policyTools: the change-policy gate for authors (check, ticket_*) and the reviewer role (review_*).
func (a *API) policyTools(s *mcp.Server) {
	mcptools.Tool(s, "policy_check", "Ask the hub policy whether a command would be allowed on a machine (dry_run=true: not logged). Returns allow, reason, matched rules and failures.",
		func(ctx context.Context, p *store.Principal, in changepolicy.Ask) (any, error) {
			if in.Tool == "" {
				in.Tool = "hub:exec"
			}
			return a.Policy.Decide(ctx, p, in)
		})
	mcptools.Tool(s, "ticket_create", "File a change ticket (draft) for a change the policy gates: id, title, machine, the exact command, fields (topology, blast_radius, snapshot...) and lists (research, verification, rollback).",
		func(ctx context.Context, p *store.Principal, in changepolicy.TicketIn) (any, error) {
			return a.Policy.Create(ctx, p, "", in)
		})
	mcptools.Tool(s, "ticket_update", "Edit your ticket (send its current version). The content hash changes, so earlier reviews go stale.",
		func(ctx context.Context, p *store.Principal, in changepolicy.TicketIn) (any, error) {
			return a.Policy.Update(ctx, p, "", in)
		})
	mcptools.Tool(s, "ticket_evidence_add", "Attach evidence (snapshot, topology, research, output) to your ticket under the absolute path its fields name. Never include secrets.",
		func(ctx context.Context, p *store.Principal, in changepolicy.EvidenceIn) (any, error) {
			return a.Policy.AddEvidence(ctx, p, "", in)
		})
	mcptools.Tool(s, "ticket_submit", "Send your draft ticket to the review queue. Then run the change with exec {ticket} once it is passed.",
		func(ctx context.Context, p *store.Principal, in ticketIDIn) (any, error) {
			return a.Policy.Submit(ctx, p, "", in.Ticket)
		})
	mcptools.Tool(s, "review_queue", "Reviewer role: change tickets waiting for review that you did not author.",
		func(ctx context.Context, p *store.Principal, in queueIn) (any, error) {
			return a.Policy.List(ctx, in.Status, p.ID)
		})
	mcptools.Tool(s, "review_ticket", "Reviewer role: one ticket with its evidence, reviews and what the policy would decide now. Note its sha256 for review_submit.",
		func(ctx context.Context, p *store.Principal, in ticketIDIn) (any, error) {
			d, err := a.Policy.Get(ctx, in.Ticket)
			if err != nil {
				return nil, err
			}
			m := ""
			if d.Ticket.Machine != nil {
				m = *d.Ticket.Machine
			}
			pre, _ := a.Policy.Decide(ctx, p, changepolicy.Ask{Tool: "hub:exec", Command: d.Ticket.Command, Machine: m, TicketID: in.Ticket, DryRun: true})
			return map[string]any{"ticket": d.Ticket, "evidence": d.Evidence, "reviews": d.Reviews, "preview": pre}, nil
		})
	mcptools.Tool(s, "review_submit", "Reviewer role: submit PASS, BLOCK or FINDINGS with a summary and findings, bound to the ticket_sha256 you reviewed. You can't review your own ticket.",
		func(ctx context.Context, p *store.Principal, in changepolicy.ReviewIn) (any, error) {
			return a.Policy.Review(ctx, p, "", in)
		})
}
