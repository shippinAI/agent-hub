package hubtest

import (
	"context"
	"testing"
	"time"
)

func (e *env) usage(who string, body map[string]any) resp {
	return e.call(who, "POST", "/usage", body)
}

func usageRow(r resp) map[string]any { m, _ := r.Body["usage"].(map[string]any); return m }

// TestUsageLedgerAndBudgets: usage is priced from the catalog by kind of token, deduplicated, attributed to the
// task and lease, append-only; a used-up budget stops new claims for its scope until a human raises it.
func TestUsageLedgerAndBudgets(t *testing.T) {
	e := setup(t)
	price := map[string]any{"id": "claude-test-2026", "provider_id": "anthropic-api", "model": "claude-test",
		"input_micros_per_mtok": 3_000_000, "cache_read_micros_per_mtok": 300_000, "output_micros_per_mtok": 15_000_000,
		"effective_from": "2026-01-01T00:00:00Z", "source": "user"}
	if r := e.call("codex", "PUT", "/prices/claude-test-2026", price); r.code() != "humans_only" {
		t.Fatalf("agent edits prices: %v", r.Body)
	}
	if r := e.call("jesse", "PUT", "/prices/claude-test-2026", price); r.Status != 200 {
		t.Fatalf("put price: %d %v", r.Status, r.Body)
	}

	id := e.newTask("jesse", "costed task", nil)
	p := "/tasks/" + itoa(id)
	l := e.call("codex", "POST", p+"/claim", map[string]any{})
	f := num(l.Body["fence"])
	turn := map[string]any{"task_id": id, "fence": f, "provider_id": "anthropic-api", "model": "claude-test", "billing_kind": "api_key",
		"input_tokens": 1_000_000, "cache_read_tokens": 400_000, "output_tokens": 100_000, "source": "stream", "external_ref": "turn-1"}
	r := e.usage("codex", turn)
	u := usageRow(r)
	// 600k uncached x $3 + 400k cache read x $0.30 + 100k output x $15 = $3.42
	if r.Status != 200 || num(u["cost_micros"]) != 3_420_000 || u["cost_basis"] != "computed" || num(u["fence"]) != f {
		t.Fatalf("priced usage: %d %v", r.Status, r.Body)
	}
	if again := usageRow(e.usage("codex", turn)); again["duplicate"] != true || num(again["id"]) != num(u["id"]) {
		t.Errorf("retry counted twice: %v", again)
	}
	for _, bad := range []map[string]any{
		{"provider_id": "anthropic-api", "billing_kind": "api_key", "source": "stream", "input_tokens": 1, "cache_read_tokens": 2},
		{"provider_id": "anthropic-api", "billing_kind": "api_key", "source": "stream", "cost_micros": 5, "cost_basis": "computed"},
		{"provider_id": "Bad Provider", "billing_kind": "api_key", "source": "stream"},
		{"provider_id": "anthropic-api", "billing_kind": "free", "source": "stream"},
	} {
		if r := e.usage("codex", bad); r.code() != "invalid" {
			t.Errorf("accepted %v: %v", bad, r.Body)
		}
	}
	// A subscription turn with the client's own estimate and no catalog price.
	if r := e.usage("codex", map[string]any{"task_id": id, "provider_id": "anthropic-claude-code", "model": "opus", "billing_kind": "subscription",
		"input_tokens": 5000, "output_tokens": 800, "cost_micros": 50_000, "cost_basis": "client_estimate", "source": "stream",
		"occurred_at": time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)}); r.Status != 200 || usageRow(r)["cost_basis"] != "client_estimate" {
		t.Fatalf("estimate usage: %v", r.Body)
	}
	// Unpriced: tokens count, cost unknown.
	e.usage("codex", map[string]any{"provider_id": "openai-api", "model": "unknown", "billing_kind": "api_key", "input_tokens": 10, "source": "manual"})

	det := e.call("jesse", "GET", p, nil).Body["usage"].(map[string]any)
	if num(det["cost_micros"]) != 3_470_000 || num(det["events"]) != 2 || num(det["estimate_micros"]) != 50_000 {
		t.Errorf("task usage: %v", det)
	}
	tot := e.call("jesse", "GET", "/usage?group_by=principal", nil).Body["totals"].([]any)
	if len(tot) != 1 || num(tot[0].(map[string]any)["unpriced_events"]) != 1 {
		t.Errorf("by principal: %v", tot)
	}

	// Budgets: humans only; used up -> no new claims for that principal; raising it lets work resume.
	budget := map[string]any{"id": "codex-daily", "scope_kind": "principal", "scope_id": "codex", "metric": "cost_micros",
		"period": "day", "limit_value": 3_000_000, "enabled": true}
	if r := e.call("codex", "PUT", "/budgets/codex-daily", budget); r.code() != "humans_only" {
		t.Fatalf("agent edits budget: %v", r.Body)
	}
	if r := e.call("jesse", "PUT", "/budgets/codex-daily", budget); r.Status != 200 {
		t.Fatalf("put budget: %d %v", r.Status, r.Body)
	}
	r = e.usage("codex", map[string]any{"task_id": id, "provider_id": "anthropic-api", "model": "claude-test", "billing_kind": "api_key",
		"input_tokens": 1000, "source": "stream"})
	if bs, _ := r.Body["budgets"].([]any); len(bs) != 1 || bs[0].(map[string]any)["over"] != true {
		t.Fatalf("budget status on record: %v", r.Body)
	}
	e.usage("codex", map[string]any{"provider_id": "anthropic-api", "model": "claude-test", "billing_kind": "api_key", "input_tokens": 1000, "source": "stream"})
	var alerts int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM events WHERE action='budget.exceeded'`).Scan(&alerts); err != nil || alerts != 1 {
		t.Errorf("budget.exceeded fired %d times (%v)", alerts, err)
	}
	other := e.newTask("jesse", "next task", nil)
	if r := e.call("codex", "POST", "/tasks/"+itoa(other)+"/claim", map[string]any{}); r.code() != "budget_exceeded" {
		t.Fatalf("claim over budget: %v", r.Body)
	}
	if r := e.call("codex", "POST", "/claim-next", map[string]any{}); r.code() != "budget_exceeded" {
		t.Fatalf("claim_next over budget: %v", r.Body)
	}
	if r := e.call("cursor", "POST", "/tasks/"+itoa(other)+"/claim", map[string]any{}); r.Status != 200 {
		t.Fatalf("another agent is not limited by codex's budget: %v", r.Body)
	}
	e.call("cursor", "POST", "/tasks/"+itoa(other)+"/release", map[string]any{"fence": e.call("jesse", "GET", "/tasks/"+itoa(other), nil).Body["task"].(map[string]any)["fence"], "reason": "x"})
	budget["limit_value"], budget["version"] = 10_000_000, 1
	if r := e.call("jesse", "PUT", "/budgets/codex-daily", budget); r.Status != 200 {
		t.Fatalf("raise budget: %v", r.Body)
	}
	if r := e.call("codex", "POST", "/tasks/"+itoa(other)+"/claim", map[string]any{}); r.Status != 200 {
		t.Fatalf("claim after raise: %v", r.Body)
	}

	// Over MCP.
	cs := e.mcpSession("codex")
	if out, isErr := callTool(t, cs, "record_usage", map[string]any{"provider_id": "anthropic-api", "model": "claude-test",
		"billing_kind": "api_key", "output_tokens": 1000, "source": "stream", "external_ref": "mcp-1"}); isErr {
		t.Fatalf("mcp record_usage: %v", out)
	}
	if out, isErr := callTool(t, cs, "usage_summary", map[string]any{"group_by": "model", "mine": true}); isErr || len(out["budgets"].([]any)) != 1 {
		t.Fatalf("mcp usage_summary: %v", out)
	}

	ctx := context.Background()
	if _, err := e.pool.Exec(ctx, `UPDATE usage_events SET cost_micros=0`); err == nil {
		t.Error("UPDATE on usage_events must be refused")
	}
	if r := e.call("jesse", "DELETE", "/prices/claude-test-2026", nil); r.Status != 200 {
		t.Errorf("delete price: %d %v", r.Status, r.Body)
	}
	var kept int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM usage_events WHERE cost_basis='computed' AND price_id IS NULL`).Scan(&kept); err != nil || kept < 1 {
		t.Errorf("deleting a price must keep the priced events: %d %v", kept, err)
	}
	if v := e.call("codex", "GET", "/events/verify", nil); v.Body["ok"] != true {
		t.Errorf("chain: %v", v.Body)
	}
}
