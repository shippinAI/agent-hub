package hubtest

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// N agents racing claim_next must never receive the same task.
func TestClaimNextRaceNeverDoubleClaims(t *testing.T) {
	e := setup(t)
	const agents, tasks = 20, 60
	e.setSetting("wip_limit_default", 0) // unlimited, so each agent keeps claiming
	for i := 0; i < agents; i++ {
		id := fmt.Sprintf("racer-%02d", i)
		r := e.call("jesse", "POST", "/principals", map[string]any{"id": id, "display_name": id, "kind": "cli",
			"roles": []string{"worker"}, "enabled": true})
		if r.Status != 200 {
			t.Fatalf("create %s: %v", id, r.Body)
		}
		tr := e.call("jesse", "POST", "/principals/"+id+"/tokens", map[string]any{"name": "race"})
		e.tokens[id] = tr.Body["token"].(string)
	}
	for i := 0; i < tasks; i++ {
		e.newTask("jesse", fmt.Sprintf("race task %d", i), nil)
	}
	var mu sync.Mutex
	got := map[int64][]string{}
	fences := map[int64]bool{}
	var wg sync.WaitGroup
	for i := 0; i < agents; i++ {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			for {
				r := e.call(id, "POST", "/claim-next", map[string]any{"ttl_seconds": 600})
				if r.Status != 200 {
					t.Errorf("%s: %d %v", id, r.Status, r.Body)
					return
				}
				if r.Body["claimed"] != true {
					return
				}
				tid, f := num(taskOf(r)["id"]), num(r.Body["fence"])
				mu.Lock()
				got[tid] = append(got[tid], id)
				if fences[f] {
					t.Errorf("fence %d handed out twice", f)
				}
				fences[f] = true
				mu.Unlock()
			}
		}(fmt.Sprintf("racer-%02d", i))
	}
	wg.Wait()
	if len(got) != tasks {
		t.Fatalf("claimed %d distinct tasks, want %d", len(got), tasks)
	}
	for tid, who := range got {
		if len(who) != 1 {
			t.Errorf("task %d claimed by %v", tid, who)
		}
	}
	var n int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM events WHERE action='task.claimed'`).Scan(&n)
	if n != tasks {
		t.Errorf("task.claimed events = %d, want %d", n, tasks)
	}
	t.Logf("%d agents claimed %d tasks: no task given twice, %d unique fences", agents, len(got), len(fences))
}
