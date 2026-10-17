package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

const genesis = "0000000000000000000000000000000000000000000000000000000000000000"

// canonical re-encodes JSON with sorted keys and verbatim numbers, so jsonb round-trips hash the same.
func canonical(raw []byte) ([]byte, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = []byte("{}")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

func eventHash(prev string, e Event) string {
	b, _ := json.Marshal(struct {
		Seq       int64           `json:"seq"`
		At        string          `json:"at"`
		Actor     string          `json:"actor"`
		Action    string          `json:"action"`
		TaskID    *int64          `json:"task_id"`
		Data      json.RawMessage `json:"data"`
		Fence     *int64          `json:"fence"`
		RequestID *string         `json:"request_id"`
	}{e.Seq, e.At.UTC().Format("2006-01-02T15:04:05.000000Z"), e.Actor, e.Action, e.TaskID, e.Data, e.Fence, e.RequestID})
	sum := sha256.Sum256(append([]byte(prev+"\n"), b...))
	return hex.EncodeToString(sum[:])
}

// Event appends one audit row in the caller's transaction (serialised by an advisory lock).
func (w *W) Event(action string, taskID *int64, fence *int64, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	e := Event{Actor: w.P.ID, Action: action, TaskID: taskID, Fence: fence}
	if e.Data, err = canonical(raw); err != nil {
		return err
	}
	if w.ReqID != "" {
		e.RequestID = &w.ReqID
	}
	if _, err := w.exec(`SELECT pg_advisory_xact_lock(7474)`); err != nil {
		return err
	}
	prev := genesis
	var last int64
	err = w.row(`SELECT seq, hash, clock_timestamp() FROM events ORDER BY seq DESC LIMIT 1`).Scan(&last, &prev, &e.At)
	if err != nil {
		prev, last = genesis, 0
		if err := w.row(`SELECT clock_timestamp()`).Scan(&e.At); err != nil {
			return err
		}
	}
	e.Seq, e.At, e.PrevHash = last+1, e.At.Truncate(time.Microsecond), prev
	e.Hash = eventHash(prev, e)
	_, err = w.exec(`INSERT INTO events(seq,at,actor,action,task_id,data,fence,request_id,prev_hash,hash)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, e.Seq, e.At, e.Actor, e.Action, e.TaskID, e.Data, e.Fence, e.RequestID, e.PrevHash, e.Hash)
	if err != nil {
		return err
	}
	_, err = w.exec(`SELECT pg_notify('hub_events', $1)`, strconv.FormatInt(e.Seq, 10))
	return err
}

// EventFilter selects a page of the log.
type EventFilter struct {
	AfterSeq int64
	TaskID   int64
	Actor    string
	Action   string
	Limit    int
	Desc     bool
}

// Events lists events in seq order (or newest first with Desc).
func (s *Store) Events(ctx context.Context, f EventFilter) ([]Event, error) {
	if f.Limit <= 0 || f.Limit > 1000 {
		f.Limit = 200
	}
	order := "ASC"
	if f.Desc {
		order = "DESC"
	}
	rows, err := s.DB.Query(ctx, `SELECT seq,at,actor,action,task_id,data,fence,request_id,prev_hash,hash FROM events
		WHERE seq > $1 AND ($2=0 OR task_id=$2) AND ($3='' OR actor=$3) AND ($4='' OR action LIKE $4 || '%')
		ORDER BY seq `+order+` LIMIT $5`, f.AfterSeq, f.TaskID, f.Actor, strings.TrimSuffix(f.Action, "*"), f.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.Seq, &e.At, &e.Actor, &e.Action, &e.TaskID, &e.Data, &e.Fence, &e.RequestID, &e.PrevHash, &e.Hash); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
