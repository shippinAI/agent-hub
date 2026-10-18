package changepolicy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"

	"agenthub/internal/access"
	"agenthub/internal/policyrules"
	"agenthub/internal/store"
	"devagotchi/bridge/scope"
)

// Ask is one decision request from a hook client (gold-policy), the UI try-it box or the bridge path.
type Ask struct {
	Event     string   `json:"event,omitempty"`
	Tool      string   `json:"tool"`
	Command   string   `json:"command"`
	Paths     []string `json:"paths,omitempty"`
	Hosts     []string `json:"hosts,omitempty"`
	Machine   string   `json:"machine,omitempty"`
	TicketID  string   `json:"ticket_id,omitempty"`
	ViaRunner bool     `json:"via_runner,omitempty"`
	DryRun    bool     `json:"dry_run,omitempty" jsonschema:"evaluate without logging and without binding the ticket to you"`
}

// Answer is the decision plus its log id.
type Answer struct {
	DecisionID string               `json:"decision_id"`
	Tenant     string               `json:"tenant_id"`
	DocSHA     string               `json:"policy_sha256"`
	Allow      bool                 `json:"allow"`
	Reason     string               `json:"reason"`
	Decision   policyrules.Decision `json:"decision"`
}

// Decide evaluates one request against the current document of the machine's tenant and appends policy.decision
// to the hash chain (argv as a fingerprint only). Any error (no doc, DB) returns an error: callers fail closed.
func (s *Service) Decide(ctx context.Context, p *store.Principal, a Ask) (Answer, error) {
	out, err := s.St.Write(ctx, p, "", "policy_decide", nil, func(w *store.W) (any, error) {
		var ans Answer
		ans.Tenant = tenantOf(w, a.Machine)
		c, err := s.current(w, ans.Tenant)
		if err != nil {
			return nil, err
		}
		hosts := append([]string{}, a.Hosts...)
		if a.Machine != "" {
			hosts = append(hosts, a.Machine) // region globs match machine ids named after their site
		}
		eng := &policyrules.Engine{Doc: c.doc, Facts: dbFacts{w: w, caller: p.ID, command: a.Command, machine: a.Machine, use: !a.DryRun}}
		d := eng.Evaluate(policyrules.Request{Event: a.Event, Source: "agent-hub", Tool: a.Tool, Command: a.Command, Paths: a.Paths,
			Hosts: hosts, Principal: p.ID, TicketID: a.TicketID, ViaRunner: a.ViaRunner})
		ans.Allow, ans.Reason, ans.Decision, ans.DocSHA, ans.DecisionID = d.Allow, d.Reason(), d, c.sha, newID()
		if a.DryRun {
			return ans, nil
		}
		fails := []map[string]any{}
		for _, f := range d.Failures {
			fails = append(fails, map[string]any{"rule": f.Rule, "why": f.Why})
		}
		return ans, w.Event("policy.decision", nil, nil, map[string]any{"decision_id": ans.DecisionID, "tenant": ans.Tenant, "policy_version": d.Version,
			"policy_sha256": c.sha, "tool": a.Tool, "fp": access.Fingerprint(strings.Fields(a.Command)), "machine": a.Machine, "allow": d.Allow,
			"matched": d.Matched, "failures": fails, "classes": d.Classes, "regions": d.Regions, "ticket": d.Ticket})
	})
	if err != nil {
		return Answer{}, err
	}
	return out.(Answer), nil
}

// Gate adapts the service to the bridge authorize path (access.PolicyGate).
type Gate struct{ S *Service }

// Decide maps a bridge call to an Ask: tool hub:<op>, the argv (or the paths for fs ops) as the command.
func (g Gate) Decide(ctx context.Context, p *store.Principal, machine string, c scope.Call) (access.PolicyVerdict, error) {
	cmd := strings.Join(c.Argv, " ")
	if len(c.Argv) == 0 {
		cmd = strings.Join(c.Paths, " ")
	}
	a, err := g.S.Decide(ctx, p, Ask{Tool: "hub:" + c.Op, Command: cmd, Paths: c.Paths, Hosts: c.Hosts, Machine: machine, TicketID: c.Ticket})
	if err != nil {
		return access.PolicyVerdict{}, err
	}
	return access.PolicyVerdict{Allow: a.Allow, Reason: a.Reason, DecisionID: a.DecisionID, Version: a.Decision.Version}, nil
}

func newID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
