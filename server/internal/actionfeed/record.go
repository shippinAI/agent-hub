package actionfeed

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"agenthub/internal/auditkeep"
	"agenthub/internal/store"
)

// Recorder writes action events: category switch, redaction, output caps, partition-on-demand, NOTIFY.
type Recorder struct {
	St   *store.Store
	Keep *auditkeep.Service

	mu     sync.Mutex
	scrub  Scrubber
	loaded time.Time
}

func (r *Recorder) scrubber(ctx context.Context) Scrubber {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.loaded.IsZero() || time.Since(r.loaded) > 5*time.Second {
		set, _ := r.St.LoadSettings(ctx)
		r.scrub, r.loaded = NewScrubber(set.Strings("secret_patterns"), set.Strings("audit_redact_keys")), time.Now()
	}
	return r.scrub
}

// Record stores e unless its category is disabled; it returns the stored event (ID 0 = skipped).
// Recording never blocks the action it describes: callers log the error and carry on.
func (r *Recorder) Record(ctx context.Context, e Event) (Event, error) {
	if e.Category == "" {
		e.Category = CategoryFor(e.Op)
	}
	c, ok := r.Keep.Lookup(ctx, e.Category)
	if !ok {
		e.Category = "tool"
		if c, ok = r.Keep.Lookup(ctx, e.Category); !ok {
			return e, nil
		}
	}
	if !c.Enabled {
		return Event{}, nil
	}
	r.clean(ctx, &e, c)
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	e.At = e.At.Truncate(time.Microsecond)
	data, err := json.Marshal(e)
	if err != nil {
		return e, err
	}
	ins := func() error {
		return r.St.DB.QueryRow(ctx, `WITH ins AS (INSERT INTO action_events(at, category, kind, call_id, actor, machine, status, data)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id) SELECT id, pg_notify('hub_actions', id::text) FROM ins`,
			e.At, e.Category, e.Kind, e.CallID, e.Actor, e.Machine, e.Status, data).Scan(&e.ID, new(any))
	}
	if err = ins(); err != nil && strings.Contains(err.Error(), "no partition of relation") {
		if err = r.Keep.EnsureDay(ctx, e.Category, e.At); err == nil {
			err = ins()
		}
	}
	return e, err
}

// clean applies redaction and the category's output policy in place.
func (r *Recorder) clean(ctx context.Context, e *Event, c auditkeep.Category) {
	s := r.scrubber(ctx)
	n, k := 0, 0
	e.Argv, k = s.Argv(e.Argv)
	n += k
	for i := range e.Paths {
		e.Paths[i], k = s.Text(e.Paths[i])
		n += k
	}
	e.Args, k = s.JSON(e.Args)
	n += k
	for _, f := range []*string{&e.Cwd, &e.Diff, &e.Stdout, &e.Stderr, &e.Error} {
		*f, k = s.Text(*f)
		n += k
	}
	e.Redactions += n
	if !c.CaptureOutput {
		e.Stdout, e.Stderr, e.Diff = "", "", ""
		return
	}
	max := c.MaxOutputBytes
	cut := func(v string, tail bool) string {
		if max <= 0 || len(v) <= max {
			return v
		}
		e.Truncated = true
		if tail {
			return v[len(v)-max:]
		}
		return v[:max]
	}
	e.Stdout, e.Stderr, e.Diff = cut(e.Stdout, true), cut(e.Stderr, true), cut(e.Diff, false)
}

// ParseID reads an id query parameter; bad or empty = 0.
func ParseID(v string) int64 { n, _ := strconv.ParseInt(v, 10, 64); return n }
