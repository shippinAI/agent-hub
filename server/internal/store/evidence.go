package store

import (
	"context"
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"time"
)

// evidenceKind is the shape of an evidence kind (data, e.g. test_run, ci_check, eval, review).
var evidenceKind = regexp.MustCompile(`^[a-z][a-z0-9_.-]{1,40}$`)

var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Evidence is one append-only proof row. Fence is the task's fence when it was recorded: the gate only
// counts evidence produced under the current lease.
type Evidence struct {
	ID        int64           `json:"id"`
	TaskID    int64           `json:"task_id"`
	Kind      string          `json:"kind"`
	Status    string          `json:"status"`
	Summary   string          `json:"summary"`
	Ref       string          `json:"ref"`
	SHA256    *string         `json:"sha256"`
	Data      json.RawMessage `json:"data"`
	Author    string          `json:"author"`
	Fence     int64           `json:"fence"`
	CreatedAt time.Time       `json:"created_at"`
}

// EvidenceInput is add_evidence's input. A supplied fence must be current (the holder proving its own work);
// without one, a non-holder (CI, a reviewer bot) may attach if policy allows task.evidence:<kind>.
type EvidenceInput struct {
	Kind    string          `json:"kind"`
	Status  string          `json:"status"`
	Summary string          `json:"summary,omitempty"`
	Ref     string          `json:"ref,omitempty"`
	SHA256  string          `json:"sha256,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
	Fence   *int64          `json:"fence,omitempty"`
}

const evidenceCols = `id,task_id,kind,status,summary,ref,sha256,data,author,fence,created_at`

func scanEvidence(r interface{ Scan(...any) error }) (Evidence, error) {
	var e Evidence
	err := r.Scan(&e.ID, &e.TaskID, &e.Kind, &e.Status, &e.Summary, &e.Ref, &e.SHA256, &e.Data, &e.Author, &e.Fence, &e.CreatedAt)
	return e, err
}

// AddEvidence appends a proof row, stamped with the task's current fence, and records it in the event chain.
func (s *Store) AddEvidence(ctx context.Context, p *Principal, reqID string, taskID int64, in EvidenceInput) (any, error) {
	return s.write(ctx, p, reqID, "add_evidence", []any{taskID, in}, func(w *W) (any, error) {
		t, err := getTask(ctx, w.tx, taskID, true)
		if err != nil {
			return nil, err
		}
		if in.Fence != nil && *in.Fence != t.Fence {
			e := fail(409, "stale_fence", "fence %d is not current", *in.Fence)
			e.Current = map[string]any{"fence": t.Fence, "holder": t.Holder}
			return nil, e
		}
		in.Kind, in.Status, in.SHA256 = strings.TrimSpace(in.Kind), strings.TrimSpace(in.Status), strings.ToLower(strings.TrimSpace(in.SHA256))
		switch {
		case !evidenceKind.MatchString(in.Kind):
			return nil, errInvalid("evidence kind must match %s", evidenceKind)
		case in.Status != "pass" && in.Status != "fail":
			return nil, errInvalid("evidence status must be pass or fail")
		case len(in.Summary) > 4000:
			return nil, errInvalid("evidence summary is limited to 4000 chars")
		case len(in.Ref) > 2000:
			return nil, errInvalid("evidence ref is limited to 2000 chars")
		case in.SHA256 != "" && !sha256Hex.MatchString(in.SHA256):
			return nil, errInvalid("sha256 must be 64 lowercase hex characters")
		}
		data := []byte(in.Data)
		if len(data) == 0 {
			data = []byte("{}")
		} else if !json.Valid(data) || data[0] != '{' {
			return nil, errInvalid("evidence data must be a JSON object")
		}
		if err := w.can("task.evidence:"+in.Kind, resOf(t)); err != nil {
			return nil, err
		}
		var sha *string
		if in.SHA256 != "" {
			sha = &in.SHA256
		}
		ev, err := scanEvidence(w.row(`INSERT INTO task_evidence(task_id,kind,status,summary,ref,sha256,data,author,fence)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+evidenceCols,
			t.ID, in.Kind, in.Status, in.Summary, in.Ref, sha, data, p.ID, t.Fence))
		if err != nil {
			return nil, err
		}
		if err := w.Event("evidence.added", &t.ID, &t.Fence, map[string]any{"evidence_id": ev.ID, "kind": ev.Kind,
			"status": ev.Status, "ref": ev.Ref, "sha256": ev.SHA256}); err != nil {
			return nil, err
		}
		return ev, nil
	})
}

// Evidence lists a task's evidence oldest first.
func (s *Store) Evidence(ctx context.Context, taskID int64) ([]Evidence, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+evidenceCols+` FROM task_evidence WHERE task_id=$1 ORDER BY id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Evidence{}
	for rows.Next() {
		e, err := scanEvidence(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// evidenceGate is the code rule behind lane.requires_evidence: for every required kind, the newest row
// recorded under the task's current fence must exist and pass. A later fail supersedes an earlier pass.
func (w *W) evidenceGate(t Task, lane Lane) error {
	if len(lane.RequiresEvidence) == 0 {
		return nil
	}
	rows, err := w.tx.Query(w.ctx, `SELECT DISTINCT ON (kind) kind, status FROM task_evidence
		WHERE task_id=$1 AND fence=$2 AND kind = ANY($3) ORDER BY kind, id DESC`, t.ID, t.Fence, lane.RequiresEvidence)
	if err != nil {
		return err
	}
	latest := map[string]string{}
	for rows.Next() {
		var k, st string
		if err := rows.Scan(&k, &st); err != nil {
			rows.Close()
			return err
		}
		latest[k] = st
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	missing, failing := []string{}, []string{}
	for _, k := range lane.RequiresEvidence {
		switch latest[k] {
		case "pass":
		case "fail":
			failing = append(failing, k)
		default:
			missing = append(missing, k)
		}
	}
	if len(missing) == 0 && len(failing) == 0 {
		return nil
	}
	e := fail(409, "evidence_missing", "lane %q needs passing evidence for this lease: missing %v, failing %v",
		lane.Active, missing, failing)
	e.Current = map[string]any{"required": lane.RequiresEvidence, "missing": missing, "failing": failing, "fence": t.Fence}
	return e
}

// cleanAcceptance trims criteria, drops blanks and caps the list (20 items, 500 chars each).
func cleanAcceptance(in []string) ([]string, error) {
	out := []string{}
	for _, c := range in {
		c = strings.TrimSpace(c)
		if c == "" || slices.Contains(out, c) {
			continue
		}
		if len(c) > 500 {
			return nil, errInvalid("an acceptance criterion is limited to 500 chars")
		}
		out = append(out, c)
	}
	if len(out) > 20 {
		return nil, errInvalid("at most 20 acceptance criteria")
	}
	return out, nil
}
