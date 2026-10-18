package hubtest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"agenthub/internal/actionfeed"
	"agenthub/internal/auditkeep"
)

// fill records n proc events spread over the last `days` days, each with outBytes of output.
func (e *env) fill(n, days, outBytes int) {
	e.t.Helper()
	now := time.Now().UTC()
	out := strings.Repeat("x", outBytes)
	for i := 0; i < n; i++ {
		at := now.Add(-time.Duration(days*24) * time.Hour * time.Duration(n-1-i) / time.Duration(max(n-1, 1)))
		if _, err := e.api.Feed.Record(context.Background(), actionfeed.Event{Category: "proc", Kind: actionfeed.KindFinished, At: at,
			Actor: "codex", Op: "exec", Argv: []string{"echo", "n"}, Status: "ok", Stdout: out}); err != nil {
			e.t.Fatal(err)
		}
	}
}

func (e *env) putCat(c auditkeep.Category) {
	e.t.Helper()
	if r := e.call("jesse", "PUT", "/audit/categories/"+c.Category, c); r.Status != 200 {
		e.t.Fatalf("put category: %v", r.Body)
	}
}

func (e *env) procRows() int64 {
	var n int64
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM action_events WHERE category='proc'`).Scan(&n)
	return n
}

// Time retention drops whole daily partitions after archiving them into verifiable, chained segments.
func TestRetentionArchivesThenDropsPartitions(t *testing.T) {
	e := setup(t)
	dir := t.TempDir()
	e.setSetting("audit_archive_dir", dir)
	e.putCat(auditkeep.Category{Category: "proc", Enabled: true, RetainDays: 2, Archive: true, CaptureOutput: true, MaxOutputBytes: 1024})
	e.fill(300, 6, 64)
	r := e.call("jesse", "POST", "/audit/prune", nil)
	if r.Status != 200 {
		t.Fatalf("prune: %v", r.Body)
	}
	if d, _ := r.Body["dropped"].([]any); len(d) < 3 {
		t.Fatalf("expected old partitions dropped: %v", r.Body)
	}
	var oldest time.Time
	_ = e.pool.QueryRow(context.Background(), `SELECT min(at) FROM action_events WHERE category='proc'`).Scan(&oldest)
	if time.Since(oldest) > 4*24*time.Hour {
		t.Errorf("rows older than retention survived: %v", oldest)
	}
	segs, _ := filepath.Glob(filepath.Join(dir, "proc", "*.manifest.json"))
	if len(segs) == 0 {
		t.Fatal("no archive segments written")
	}
	for _, m := range segs {
		var seg auditkeep.Segment
		b, _ := os.ReadFile(m)
		_ = json.Unmarshal(b, &seg)
		gz, _ := os.ReadFile(strings.TrimSuffix(m, ".manifest.json") + ".jsonl.gz")
		if err := auditkeep.VerifySegment(gz, seg, nil); err != nil {
			t.Errorf("segment %s: %v", m, err)
		}
	}
	if err := e.api.Keep.VerifyChainOfSegments(context.Background()); err != nil {
		t.Error(err)
	}
	if rep, _ := e.st.VerifyChain(context.Background()); !rep.OK {
		t.Errorf("governance chain broken after prune: %+v", rep)
	}
	ev := fmtAny(e.call("jesse", "GET", "/events?action=audit.", nil).Body)
	if !strings.Contains(ev, "audit.archived") || !strings.Contains(ev, "audit.pruned") {
		t.Errorf("prune not anchored in the chain: %s", ev)
	}
	if r := e.call("codex", "POST", "/audit/prune", nil); r.Status != 403 {
		t.Errorf("agent pruned: %v", r.Body)
	}
}

// Row limits trim the open partition too, and with on_failure=keep nothing unarchived is lost.
func TestRetentionRowCapAndArchiveFailure(t *testing.T) {
	e := setup(t)
	e.putCat(auditkeep.Category{Category: "proc", Enabled: true, MaxRows: 50, Archive: true, CaptureOutput: true, MaxOutputBytes: 1024})
	e.setSetting("audit_on_archive_failure", "keep") // no archive dir: archiving fails
	e.fill(200, 0, 16)
	rep, err := e.api.Keep.Prune(context.Background())
	if err != nil || len(rep.Warnings) == 0 || e.procRows() != 200 {
		t.Fatalf("keep policy lost rows: rows=%d rep=%+v err=%v", e.procRows(), rep, err)
	}
	e.setSetting("audit_on_archive_failure", "drop")
	if _, err := e.api.Keep.Prune(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := e.procRows(); n != 50 {
		t.Fatalf("row cap: %d rows", n)
	}
	e.setSetting("audit_archive_dir", t.TempDir())
	e.fill(30, 0, 16)
	r := e.call("jesse", "POST", "/audit/flush?category=proc", nil)
	if a, _ := r.Body["archived"].([]any); r.Status != 200 || len(a) == 0 {
		t.Fatalf("flush: %v", r.Body)
	}
	if n := e.procRows(); n != 80 {
		t.Fatalf("flush deleted rows: %d", n)
	}
}

// Storage never fills: under sustained writes the byte cap holds after every prune, and the on-disk size of
// the category stops growing (VACUUM makes trimmed space reusable).
func TestRetentionStorageNeverFills(t *testing.T) {
	e := setup(t)
	const capBytes = 400 << 10
	e.setSetting("audit_total_max_bytes", capBytes)
	e.putCat(auditkeep.Category{Category: "proc", Enabled: true, Archive: false, CaptureOutput: true, MaxOutputBytes: 4096})
	var peak int64
	for round := 0; round < rounds(); round++ {
		e.fill(250, 0, 2048)
		rep, err := e.api.Keep.Prune(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		var disk int64
		_ = e.pool.QueryRow(context.Background(), `SELECT coalesce(sum(pg_total_relation_size(c.oid)),0) FROM pg_class c
			WHERE c.relname LIKE 'action_events_proc_p%'`).Scan(&disk)
		var live int64
		_ = e.pool.QueryRow(context.Background(), `SELECT coalesce(sum(pg_column_size(t.*)),0) FROM action_events t WHERE category='proc'`).Scan(&live)
		t.Logf("round %d: live=%d disk=%d deleted=%d", round, live, disk, rep.Deleted)
		if live > capBytes {
			t.Fatalf("round %d: live bytes %d over cap %d", round, live, capBytes)
		}
		if round == 3 {
			peak = disk
		}
		if round > 3 && disk > peak*2 {
			t.Fatalf("round %d: disk keeps growing (%d > 2x %d)", round, disk, peak)
		}
	}
}

// rounds is 8 by default; HUB_FILL_ROUNDS raises it for a longer soak.
func rounds() int {
	if n, err := strconv.Atoi(os.Getenv("HUB_FILL_ROUNDS")); err == nil && n > 0 {
		return n
	}
	return 8
}
