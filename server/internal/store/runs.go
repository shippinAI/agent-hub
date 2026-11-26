package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Run is one agent attempt at a task under one lease, with its transcript stored in ordered chunks.
type Run struct {
	ID               int64      `json:"id"`
	TaskID           int64      `json:"task_id"`
	Fence            int64      `json:"fence"`
	Principal        string     `json:"principal_id"`
	Seat             string     `json:"seat"`
	Adapter          string     `json:"adapter"`
	Model            string     `json:"model"`
	SessionID        string     `json:"session_id"`
	Branch           string     `json:"branch"`
	HeadSHA          string     `json:"head_sha"`
	Status           string     `json:"status"`
	Reason           string     `json:"reason"`
	Turns            int        `json:"turns"`
	TranscriptBytes  int64      `json:"transcript_bytes"`
	TranscriptChunks int        `json:"transcript_chunks"`
	TranscriptSHA256 string     `json:"transcript_sha256"`
	Truncated        bool       `json:"truncated"`
	StartedAt        time.Time  `json:"started_at"`
	EndedAt          *time.Time `json:"ended_at"`
}

// RunStart opens a run on a task the caller holds.
type RunStart struct {
	Fence     int64  `json:"fence"`
	Adapter   string `json:"adapter"`
	Seat      string `json:"seat,omitempty"`
	Model     string `json:"model,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Branch    string `json:"branch,omitempty"`
}

// RunFinish closes a run. Empty fields keep what the run already has.
type RunFinish struct {
	Status    string `json:"status"`
	Reason    string `json:"reason,omitempty"`
	Turns     int    `json:"turns,omitempty"`
	Model     string `json:"model,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Branch    string `json:"branch,omitempty"`
	HeadSHA   string `json:"head_sha,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

// RunChunk is one ordered piece of a transcript (JSONL lines as the agent printed them).
type RunChunk struct {
	Seq  int    `json:"seq"`
	Body string `json:"body"`
}

const (
	runCols           = `id,task_id,fence,principal_id,seat,adapter,model,session_id,branch,head_sha,status,reason,turns,transcript_bytes,transcript_chunks,transcript_sha256,truncated,started_at,ended_at`
	maxChunk          = 256 << 10
	maxTranscriptSize = 64 << 20
)

var (
	adapterName  = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,40}$`)
	runEndStates = []string{"completed", "handed_off", "lost", "failed"}
)

func scanRun(r interface{ Scan(...any) error }) (Run, error) {
	var x Run
	err := r.Scan(&x.ID, &x.TaskID, &x.Fence, &x.Principal, &x.Seat, &x.Adapter, &x.Model, &x.SessionID, &x.Branch, &x.HeadSHA,
		&x.Status, &x.Reason, &x.Turns, &x.TranscriptBytes, &x.TranscriptChunks, &x.TranscriptSHA256, &x.Truncated, &x.StartedAt, &x.EndedAt)
	return x, err
}

func checkRunFields(model, session, branch, head, reason string) error {
	switch {
	case len(model) > 200 || len(session) > 200 || len(branch) > 255:
		return errInvalid("model and session_id are limited to 200 chars, branch to 255")
	case head != "" && !gitSHA.MatchString(head):
		return errInvalid("head_sha must be a 40-character commit sha")
	case len(reason) > 4000:
		return errInvalid("reason is limited to 4000 chars")
	}
	return nil
}

// StartRun opens a run under the caller's current lease. Runs of the task still open from an earlier lease are
// closed as lost, since their lease is gone.
func (s *Store) StartRun(ctx context.Context, p *Principal, reqID string, taskID int64, in RunStart) (any, error) {
	return s.write(ctx, p, reqID, "start_run", []any{taskID, in}, func(w *W) (any, error) {
		t, _, _, err := w.held(taskID, in.Fence)
		if err != nil {
			return nil, err
		}
		if !adapterName.MatchString(in.Adapter) {
			return nil, errInvalid("adapter must match %s", adapterName)
		}
		if len(in.Seat) > 100 {
			return nil, errInvalid("seat is limited to 100 chars")
		}
		if err := checkRunFields(in.Model, in.SessionID, in.Branch, "", ""); err != nil {
			return nil, err
		}
		if err := w.can("task.run", resOf(t)); err != nil {
			return nil, err
		}
		rows, err := w.tx.Query(ctx, `UPDATE runs SET status='lost', reason='its lease ended before the run finished', ended_at=clock_timestamp()
			WHERE task_id=$1 AND status='running' RETURNING id`, t.ID)
		if err != nil {
			return nil, err
		}
		var stale []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			stale = append(stale, id)
		}
		rows.Close()
		for _, id := range stale {
			if err := w.Event("run.finished", &t.ID, &t.Fence, map[string]any{"run_id": id, "status": "lost"}); err != nil {
				return nil, err
			}
		}
		r, err := scanRun(w.row(`INSERT INTO runs(task_id,fence,principal_id,seat,adapter,model,session_id,branch)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING `+runCols, t.ID, t.Fence, p.ID, in.Seat, in.Adapter, in.Model, in.SessionID, in.Branch))
		if err != nil {
			return nil, err
		}
		return r, w.Event("run.started", &t.ID, &t.Fence, map[string]any{"run_id": r.ID, "adapter": r.Adapter, "seat": r.Seat, "model": r.Model})
	})
}

// ownRun loads a run for update and checks the caller opened it and it is still running.
func (w *W) ownRun(id int64) (Run, error) {
	r, err := scanRun(w.row(`SELECT `+runCols+` FROM runs WHERE id=$1 FOR UPDATE`, id))
	if isNoRows(err) {
		return r, errNotFound(fmt.Sprintf("run %d", id))
	}
	if err != nil {
		return r, err
	}
	if r.Principal != w.P.ID {
		return r, fail(403, "policy_denied", "only the principal that started run %d can write to it", id)
	}
	if r.Status != "running" {
		e := fail(409, "run_finished", "run %d already finished (%s)", id, r.Status)
		e.Current = map[string]any{"status": r.Status}
		return r, e
	}
	return r, nil
}

// AppendTranscript adds chunk seq to a running run. Re-sending a chunk with the same body is a no-op, so a seat
// can retry; a different body for a stored seq is refused.
func (s *Store) AppendTranscript(ctx context.Context, p *Principal, reqID string, runID int64, c RunChunk) (any, error) {
	return s.write(ctx, p, reqID, "append_transcript", []any{runID, c.Seq, len(c.Body)}, func(w *W) (any, error) {
		r, err := w.ownRun(runID)
		if err != nil {
			return nil, err
		}
		switch {
		case c.Seq < 0:
			return nil, errInvalid("seq cannot be negative")
		case c.Body == "" || len(c.Body) > maxChunk:
			return nil, errInvalid("a chunk is 1 to %d bytes", maxChunk)
		}
		var old string
		if err := w.row(`SELECT body FROM run_chunks WHERE run_id=$1 AND seq=$2`, runID, c.Seq).Scan(&old); err == nil {
			if old != c.Body {
				return nil, fail(409, "chunk_conflict", "chunk %d of run %d was stored with different content", c.Seq, runID)
			}
			return map[string]any{"run_id": runID, "seq": c.Seq, "duplicate": true}, nil
		} else if !isNoRows(err) {
			return nil, err
		}
		if r.TranscriptBytes+int64(len(c.Body)) > maxTranscriptSize {
			e := fail(413, "transcript_too_large", "run %d's transcript would exceed %d bytes; finish it with truncated=true", runID, maxTranscriptSize)
			e.Current = map[string]any{"transcript_bytes": r.TranscriptBytes}
			return nil, e
		}
		if _, err := w.exec(`INSERT INTO run_chunks(run_id,seq,body) VALUES($1,$2,$3)`, runID, c.Seq, c.Body); err != nil {
			return nil, err
		}
		if _, err := w.exec(`UPDATE runs SET transcript_bytes=transcript_bytes+$2, transcript_chunks=transcript_chunks+1 WHERE id=$1`,
			runID, len(c.Body)); err != nil {
			return nil, err
		}
		return map[string]any{"run_id": runID, "seq": c.Seq}, nil
	})
}

// FinishRun closes a run, hashes its transcript (chunks in seq order) and logs run.finished with that hash.
// It needs no lease: a seat that lost its lease still records how the run ended.
func (s *Store) FinishRun(ctx context.Context, p *Principal, reqID string, runID int64, in RunFinish) (any, error) {
	return s.write(ctx, p, reqID, "finish_run", []any{runID, in}, func(w *W) (any, error) {
		r, err := w.ownRun(runID)
		if err != nil {
			return nil, err
		}
		in.HeadSHA = strings.ToLower(strings.TrimSpace(in.HeadSHA))
		if !slices.Contains(runEndStates, in.Status) {
			return nil, errInvalid("status must be one of %v", runEndStates)
		}
		if in.Turns < 0 {
			return nil, errInvalid("turns cannot be negative")
		}
		if err := checkRunFields(in.Model, in.SessionID, in.Branch, in.HeadSHA, in.Reason); err != nil {
			return nil, err
		}
		rows, err := w.tx.Query(ctx, `SELECT seq, body FROM run_chunks WHERE run_id=$1 ORDER BY seq`, runID)
		if err != nil {
			return nil, err
		}
		h, gap, next := sha256.New(), false, 0
		for rows.Next() {
			var seq int
			var body string
			if err := rows.Scan(&seq, &body); err != nil {
				rows.Close()
				return nil, err
			}
			if seq != next {
				gap = true
			}
			next = seq + 1
			h.Write([]byte(body))
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		sum := ""
		if r.TranscriptChunks > 0 {
			sum = hex.EncodeToString(h.Sum(nil))
		}
		out, err := scanRun(w.row(`UPDATE runs SET status=$2, reason=$3, turns=GREATEST(turns,$4),
			model=CASE WHEN $5<>'' THEN $5 ELSE model END, session_id=CASE WHEN $6<>'' THEN $6 ELSE session_id END,
			branch=CASE WHEN $7<>'' THEN $7 ELSE branch END, head_sha=CASE WHEN $8<>'' THEN $8 ELSE head_sha END,
			truncated=$9, transcript_sha256=$10, ended_at=clock_timestamp() WHERE id=$1 RETURNING `+runCols,
			runID, in.Status, in.Reason, in.Turns, in.Model, in.SessionID, in.Branch, in.HeadSHA, in.Truncated || gap, sum))
		if err != nil {
			return nil, err
		}
		return out, w.Event("run.finished", &out.TaskID, &out.Fence, map[string]any{"run_id": out.ID, "status": out.Status,
			"turns": out.Turns, "head_sha": out.HeadSHA, "transcript_sha256": out.TranscriptSHA256, "transcript_bytes": out.TranscriptBytes,
			"truncated": out.Truncated})
	})
}

// Runs lists a task's runs, oldest first, without transcripts.
func (s *Store) Runs(ctx context.Context, taskID int64) ([]Run, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+runCols+` FROM runs WHERE task_id=$1 ORDER BY id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Run{}
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RunTranscript returns a run and its chunks after afterSeq, up to about 4 MiB per page.
func (s *Store) RunTranscript(ctx context.Context, runID int64, afterSeq int) (Run, []RunChunk, bool, error) {
	r, err := scanRun(s.DB.QueryRow(ctx, `SELECT `+runCols+` FROM runs WHERE id=$1`, runID))
	if isNoRows(err) {
		return r, nil, false, errNotFound(fmt.Sprintf("run %d", runID))
	}
	if err != nil {
		return r, nil, false, err
	}
	rows, err := s.DB.Query(ctx, `SELECT seq, body FROM run_chunks WHERE run_id=$1 AND seq>$2 ORDER BY seq`, runID, afterSeq)
	if err != nil {
		return r, nil, false, err
	}
	defer rows.Close()
	out, size, more := []RunChunk{}, 0, false
	for rows.Next() {
		var c RunChunk
		if err := rows.Scan(&c.Seq, &c.Body); err != nil {
			return r, nil, false, err
		}
		if size > 0 && size+len(c.Body) > 4<<20 {
			more = true
			break
		}
		size += len(c.Body)
		out = append(out, c)
	}
	return r, out, more, rows.Err()
}
