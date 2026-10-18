package actionfeed

import (
	"context"
	"encoding/json"
)

// Filter selects events. Order "asc" (default, for tailing after AfterID) or "desc" (newest first).
type Filter struct {
	AfterID  int64
	BeforeID int64
	Category string
	CallID   string
	Actor    string
	Machine  string
	Limit    int
	Order    string
}

// List returns events matching f (limit 1..1000, default 200).
func (r *Recorder) List(ctx context.Context, f Filter) ([]Event, error) {
	if f.Limit <= 0 || f.Limit > 1000 {
		f.Limit = 200
	}
	order := "ASC"
	if f.Order == "desc" {
		order = "DESC"
	}
	rows, err := r.St.DB.Query(ctx, `SELECT id, data FROM action_events
		WHERE id > $1 AND ($2 = 0 OR id < $2) AND ($3 = '' OR category = $3) AND ($4 = '' OR call_id = $4)
		AND ($5 = '' OR actor = $5) AND ($6 = '' OR machine = $6) ORDER BY id `+order+` LIMIT $7`,
		f.AfterID, f.BeforeID, f.Category, f.CallID, f.Actor, f.Machine, f.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var id int64
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		var e Event
		if err := json.Unmarshal(raw, &e); err != nil {
			continue
		}
		e.ID = id
		out = append(out, e)
	}
	return out, rows.Err()
}

// Head is the newest event id (0 when empty).
func (r *Recorder) Head(ctx context.Context) int64 {
	var id int64
	_ = r.St.DB.QueryRow(ctx, `SELECT coalesce(max(id),0) FROM action_events`).Scan(&id)
	return id
}
