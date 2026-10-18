package policyrules

import "time"

// Ticket is a change ticket as the engine sees it. The hub loads it from change_tickets; SHA256 covers the content.
type Ticket struct {
	ID     string              `json:"id"`
	Author string              `json:"author"`
	SHA256 string              `json:"sha256"`
	Fields map[string]string   `json:"fields"`
	Lists  map[string][]string `json:"lists"`
}

// Value returns a field, or a list joined with "; " when the key is a list.
func (t *Ticket) Value(k string) string {
	if v := t.Fields[k]; v != "" {
		return v
	}
	s := ""
	for i, x := range t.Lists[k] {
		if i > 0 {
			s += "; "
		}
		s += x
	}
	return s
}

// Evidence is one captured artifact attached to a ticket (a snapshot, a topology dump, a research note).
type Evidence struct {
	Path       string    `json:"path"`
	Text       string    `json:"text"`
	CapturedAt time.Time `json:"captured_at"`
}

// Review is one verdict on a ticket. Only reviews on the ticket's current SHA256 are passed to the engine.
type Review struct {
	Reviewer string   `json:"reviewer"`
	Verdict  string   `json:"verdict"` // PASS | BLOCK | FINDINGS
	Human    bool     `json:"human"`
	Roles    []string `json:"roles"`
}

// Facts is everything a check may look at besides the request: tickets, their evidence and their reviews.
// The hub implements it over Postgres; tests use a map.
type Facts interface {
	Ticket(id string) (*Ticket, error)
	Evidence(t *Ticket, path string) (Evidence, bool)
	Reviews(t *Ticket) []Review
}

// Env holds the clock (tests pin it).
type Env struct {
	Now func() time.Time
}

func (e Env) now() time.Time {
	if e.Now == nil {
		return time.Now()
	}
	return e.Now()
}

// ValidID reports whether s is a safe rule or ticket id.
func ValidID(s string) bool { return idRe.MatchString(s) }

var idRe = re(`^[A-Za-z0-9][A-Za-z0-9._-]{2,63}$`)
