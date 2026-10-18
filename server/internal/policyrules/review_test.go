package policyrules

import (
	"errors"
	"strings"
	"testing"
	"time"
)

type mapFacts struct {
	t   *Ticket
	ev  map[string]Evidence
	rvs []Review
}

func (m mapFacts) Ticket(id string) (*Ticket, error) {
	if m.t == nil || m.t.ID != id {
		return nil, errors.New("ticket " + id + " not found")
	}
	return m.t, nil
}
func (m mapFacts) Evidence(_ *Ticket, p string) (Evidence, bool) { e, ok := m.ev[p]; return e, ok }
func (m mapFacts) Reviews(*Ticket) []Review                      { return m.rvs }

func one() *int { v := 1; return &v }

func reviewDoc() *Doc {
	return &Doc{Version: 4, Classes: map[string][]string{"system-change": {`\bsystemctl\s+restart\b`}},
		Rules: []Rule{{ID: "infra-change-review", Enabled: true, Effect: "require", When: When{Classes: []string{"system-change"}},
			Require: []Check{{Kind: "ticket"}, {Kind: "field", Field: "snapshot", FileExists: true, MaxAgeS: 3600, ContainsID: true},
				{Kind: "review", Min: one(), Role: "reviewer"}}}}}
}

func TestReviewCheck(t *testing.T) {
	now := time.Date(2026, 10, 18, 1, 0, 0, 0, time.UTC)
	tk := &Ticket{ID: "CHG-1", Author: "codex", Fields: map[string]string{"snapshot": "/s/CHG-1.txt"}}
	f := mapFacts{t: tk, ev: map[string]Evidence{"/s/CHG-1.txt": {Text: "CHG-1 state", CapturedAt: now.Add(-time.Minute)}}}
	run := func(f mapFacts, ticket string) Decision {
		e := &Engine{Doc: reviewDoc(), Facts: f, Env: Env{Now: func() time.Time { return now }}}
		return e.Evaluate(Request{Tool: "hub:exec", Command: "systemctl restart nginx", TicketID: ticket})
	}
	cases := []struct {
		name  string
		rvs   []Review
		allow bool
		why   string
	}{
		{"no reviews", nil, false, "needs 1 PASS"},
		{"self PASS ignored", []Review{{Reviewer: "codex", Verdict: "PASS", Roles: []string{"reviewer"}}}, false, "has 0"},
		{"PASS without role ignored", []Review{{Reviewer: "cursor", Verdict: "PASS"}}, false, "has 0"},
		{"reviewer PASS", []Review{{Reviewer: "clyffy-infra-auditor", Verdict: "PASS", Roles: []string{"reviewer"}}}, true, ""},
		{"BLOCK beats PASS", []Review{{Reviewer: "a1", Verdict: "PASS", Roles: []string{"reviewer"}}, {Reviewer: "a2", Verdict: "BLOCK", Roles: []string{"reviewer"}}}, false, "BLOCK"},
		{"latest verdict per reviewer", []Review{{Reviewer: "a1", Verdict: "BLOCK", Roles: []string{"reviewer"}}, {Reviewer: "a1", Verdict: "PASS", Roles: []string{"reviewer"}}}, true, ""},
		{"FINDINGS hold", []Review{{Reviewer: "a1", Verdict: "FINDINGS", Roles: []string{"reviewer"}}}, false, "FINDINGS"},
	}
	for _, c := range cases {
		f.rvs = c.rvs
		d := run(f, "CHG-1")
		if d.Allow != c.allow || (c.why != "" && !strings.Contains(d.Reason(), c.why)) {
			t.Errorf("%s: allow=%v reason=%s", c.name, d.Allow, d.Reason())
		}
	}
	f.rvs = cases[3].rvs
	if d := run(f, ""); d.Allow || !strings.Contains(d.Reason(), "needs a change ticket") {
		t.Errorf("no ticket: %s", d.Reason())
	}
	if d := run(f, "CHG-404"); d.Allow || !strings.Contains(d.Reason(), "not found") {
		t.Errorf("unknown ticket: %s", d.Reason())
	}
	old := f
	old.ev = map[string]Evidence{"/s/CHG-1.txt": {Text: "CHG-1", CapturedAt: now.Add(-2 * time.Hour)}}
	if d := run(old, "CHG-1"); d.Allow || !strings.Contains(d.Reason(), "older than") {
		t.Errorf("stale evidence: %s", d.Reason())
	}
	old.ev = nil
	if d := run(old, "CHG-1"); d.Allow || !strings.Contains(d.Reason(), "not attached") {
		t.Errorf("missing evidence: %s", d.Reason())
	}
	if d := (&Engine{Doc: reviewDoc()}).Evaluate(Request{Command: "systemctl restart x", TicketID: "CHG-1"}); d.Allow {
		t.Errorf("no facts must fail closed")
	}
}

func TestApprovalNeedsHuman(t *testing.T) {
	doc := &Doc{Regions: []Region{{Code: "WI", Hosts: []string{"wi-*"}}}, Rules: []Rule{{ID: "geo-block", Enabled: true, Effect: "require",
		When: When{Regions: []string{"WI"}}, Require: []Check{{Kind: "ticket"}, {Kind: "approval"}}}}}
	tk := &Ticket{ID: "CHG-2", Author: "clyffy"}
	for _, c := range []struct {
		rv    Review
		allow bool
	}{{Review{Reviewer: "clyffy-infra-auditor", Verdict: "PASS", Roles: []string{"reviewer"}}, false},
		{Review{Reviewer: "jesse", Verdict: "PASS", Human: true}, true}} {
		e := &Engine{Doc: doc, Facts: mapFacts{t: tk, rvs: []Review{c.rv}}}
		if d := e.Evaluate(Request{Command: "ssh wi-gw-1 uptime", TicketID: "CHG-2"}); d.Allow != c.allow {
			t.Errorf("%s: allow=%v %s", c.rv.Reviewer, d.Allow, d.Reason())
		}
	}
	if err := (&Doc{Rules: []Rule{{ID: "bad", Effect: "require", Require: []Check{{Kind: "vibes"}}}}}).Validate(); err == nil {
		t.Error("unknown check kind validated")
	}
}
