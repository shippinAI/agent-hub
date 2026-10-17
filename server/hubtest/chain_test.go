package hubtest

import (
	"context"
	"testing"
)

func TestHashChainVerifyAndAppendOnly(t *testing.T) {
	e := setup(t)
	for i := 0; i < 5; i++ {
		e.newTask("codex", "chain task", map[string]any{"labels": []string{"a", "b"}})
	}
	e.call("codex", "POST", "/tasks/1/notes", map[string]any{"body": "a note", "kind": "decision"})
	v := e.call("codex", "GET", "/events/verify", nil)
	if v.Body["ok"] != true || num(v.Body["count"]) < 6 {
		t.Fatalf("verify: %v", v.Body)
	}
	ctx := context.Background()
	if _, err := e.pool.Exec(ctx, `UPDATE events SET actor='mallory' WHERE seq=3`); err == nil {
		t.Fatal("UPDATE on events should be refused by the append-only trigger")
	}
	if _, err := e.pool.Exec(ctx, `DELETE FROM events WHERE seq=3`); err == nil {
		t.Fatal("DELETE on events should be refused")
	}
	if _, err := e.pool.Exec(ctx, `UPDATE notes SET body='x' WHERE task_id=1`); err == nil {
		t.Fatal("UPDATE on notes should be refused")
	}
	// The table owner can still disable triggers: simulate that tamper and make sure verify notices.
	if _, err := e.pool.Exec(ctx, `ALTER TABLE events DISABLE TRIGGER events_ro; UPDATE events SET data='{"title":"forged"}' WHERE seq=3;
		ALTER TABLE events ENABLE TRIGGER events_ro`); err != nil {
		t.Fatal(err)
	}
	v = e.call("codex", "GET", "/events/verify", nil)
	if v.Body["ok"] != false || num(v.Body["broken_seq"]) != 3 {
		t.Fatalf("tampered chain should fail at seq 3: %v", v.Body)
	}
	t.Logf("tamper detected: %v", v.Body["reason"])
}
