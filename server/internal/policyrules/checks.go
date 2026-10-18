package policyrules

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var placeholder = re(`(\?\?|\bTBD\b|\bTODO\b|\bXXX\b|YYYYMMDD|<[a-z _-]+>)`)

// run evaluates one check and returns a human reason when it fails ("" = pass).
func (e *Engine) run(c Check, r Request, t *Ticket) string {
	if c.Kind == "via_runner" {
		if r.ViaRunner {
			return ""
		}
		return or(c.Msg, "must run through the commit-confirm runner: gold-policy run <ticket-id>")
	}
	if t == nil {
		return "needs a change ticket: create one with ticket_create and pass its id as ticket (or GOLD_TICKET=<id>)"
	}
	switch c.Kind {
	case "ticket":
		return ""
	case "approval":
		return e.approval(c, t)
	case "review":
		return e.review(c, t)
	case "field":
		return e.field(c, t)
	case "list":
		return e.list(c, t)
	case "number":
		v, err := strconv.Atoi(t.Fields[c.Field])
		if err != nil {
			return or(c.Msg, fmt.Sprintf("field '%s' must be a number", c.Field))
		}
		if (c.Min != nil && v < *c.Min) || (c.Max != nil && v > *c.Max) {
			return or(c.Msg, fmt.Sprintf("field '%s'=%d is outside %s", c.Field, v, bounds(c)))
		}
		return ""
	}
	return fmt.Sprintf("unknown check kind %q (failing closed)", c.Kind)
}

func (e *Engine) field(c Check, t *Ticket) string {
	v := t.Fields[c.Field]
	switch {
	case v == "":
		return or(c.Msg, fmt.Sprintf("field '%s' is missing", c.Field))
	case placeholder.MatchString(v):
		return fmt.Sprintf("field '%s' still has a template placeholder", c.Field)
	case len(v) < c.MinLen:
		return or(c.Msg, fmt.Sprintf("field '%s' is too short (need %d chars)", c.Field, c.MinLen))
	case c.Pattern != "" && !re(c.Pattern).MatchString(v):
		return or(c.Msg, fmt.Sprintf("field '%s' must match %s", c.Field, c.Pattern))
	case c.Not != "" && re(c.Not).MatchString(v):
		return or(c.Msg, fmt.Sprintf("field '%s' must not match %s", c.Field, c.Not))
	}
	if c.FileExists {
		return e.file(c, t, v)
	}
	return ""
}

func (e *Engine) list(c Check, t *Ticket) string {
	items := t.Lists[c.Field]
	min := 1
	if c.Min != nil {
		min = *c.Min
	}
	if len(items) < min {
		return or(c.Msg, fmt.Sprintf("list '%s' needs at least %d item(s)", c.Field, min))
	}
	for _, it := range items {
		if c.Pattern != "" && !re(c.Pattern).MatchString(it) {
			return or(c.Msg, fmt.Sprintf("list '%s' item %q must match %s", c.Field, it, c.Pattern))
		}
		if c.FileExists && !strings.HasPrefix(it, "http://") && !strings.HasPrefix(it, "https://") {
			if why := e.file(c, t, it); why != "" {
				return why
			}
		}
	}
	return ""
}

// file resolves a path against the ticket's evidence: the hub cannot see the target machine's disk.
func (e *Engine) file(c Check, t *Ticket, p string) string {
	if !strings.HasPrefix(p, "/") {
		return fmt.Sprintf("'%s' must be an absolute path (got %q)", c.Field, p)
	}
	var ev Evidence
	ok := false
	if e.Facts != nil {
		ev, ok = e.Facts.Evidence(t, p)
	}
	if !ok {
		return or(c.Msg, fmt.Sprintf("'%s' evidence %s is not attached to the ticket", c.Field, p))
	}
	if c.MaxAgeS > 0 && e.Env.now().Sub(ev.CapturedAt) > time.Duration(c.MaxAgeS)*time.Second {
		return fmt.Sprintf("'%s' evidence %s is older than %ds; capture a fresh one", c.Field, p, c.MaxAgeS)
	}
	if c.ContainsID && !strings.Contains(ev.Text, t.ID) {
		return fmt.Sprintf("'%s' evidence %s does not mention %s", c.Field, p, t.ID)
	}
	return ""
}

func bounds(c Check) string {
	lo, hi := "-inf", "+inf"
	if c.Min != nil {
		lo = strconv.Itoa(*c.Min)
	}
	if c.Max != nil {
		hi = strconv.Itoa(*c.Max)
	}
	return "[" + lo + ", " + hi + "]"
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
