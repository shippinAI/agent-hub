package search

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"sync"
	"time"
)

// Session is one streaming search.
type Session struct {
	ID      string
	Q       Query
	Started time.Time
	mu      sync.Mutex
	hits    []Hit
	done    bool
	ended   time.Time
	err     string
	skipped int
	cancel  context.CancelFunc
}

// Status is the page header get_more_search_results returns.
type Status struct {
	ID        string `json:"sessionId"`
	Type      string `json:"searchType"`
	Pattern   string `json:"pattern"`
	Root      string `json:"path"`
	Complete  bool   `json:"isComplete"`
	Total     int    `json:"totalResults"`
	RuntimeMS int64  `json:"runtime_ms"`
	Skipped   int    `json:"skipped_large_files,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Manager keeps sessions and forgets finished ones after Keep.
type Manager struct {
	mu   sync.Mutex
	all  map[string]*Session
	Keep time.Duration
}

// NewManager makes a manager (DC keeps stopped searches readable for 5 minutes).
func NewManager() *Manager { return &Manager{all: map[string]*Session{}, Keep: 5 * time.Minute} }

// Start begins a search in the background. timeout 0 = none. Max results 0 = cap only.
func (m *Manager) Start(q Query, timeout time.Duration, cap int, h Hooks) *Session {
	m.reap()
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	s := &Session{ID: "search_" + hex.EncodeToString(b), Q: q, Started: time.Now()}
	ctx, cancel := context.WithCancel(context.Background())
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	}
	s.cancel = cancel
	limit := cap
	if q.MaxResults > 0 && q.MaxResults < cap {
		limit = q.MaxResults
	}
	h.Emit = func(hit Hit) bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		if len(s.hits) >= limit {
			return false
		}
		s.hits = append(s.hits, hit)
		return len(s.hits) < limit
	}
	h.Skipped = func() { s.mu.Lock(); s.skipped++; s.mu.Unlock() }
	m.mu.Lock()
	m.all[s.ID] = s
	m.mu.Unlock()
	go func() {
		err := Run(ctx, q, h)
		s.mu.Lock()
		s.done, s.ended = true, time.Now()
		if err != nil {
			s.err = err.Error()
		} else if ctx.Err() == context.DeadlineExceeded {
			s.err = "timeout reached; results so far are kept"
		}
		s.mu.Unlock()
		cancel()
	}()
	return s
}

// Get finds a session.
func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.all[id]
	return s, ok
}

// Stop cancels a session; its results stay readable until reaped.
func (m *Manager) Stop(id string) bool {
	s, ok := m.Get(id)
	if ok {
		s.cancel()
	}
	return ok
}

// List returns all sessions, newest first.
func (m *Manager) List() []Status {
	m.reap()
	m.mu.Lock()
	out := []Status{}
	for _, s := range m.all {
		out = append(out, s.Status())
	}
	m.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
