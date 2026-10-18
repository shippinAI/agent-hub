package search

import "time"

// Page returns hits [offset, offset+length); a negative offset reads the last -offset hits.
func (s *Session) Page(offset, length int) ([]Hit, Status) {
	s.mu.Lock()
	n := len(s.hits)
	from := offset
	if offset < 0 {
		from, length = max(0, n+offset), -offset
	}
	from = min(from, n)
	page := append([]Hit(nil), s.hits[from:min(n, from+length)]...)
	s.mu.Unlock()
	return page, s.Status()
}

// Status reports progress.
func (s *Session) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	end := time.Now()
	if s.done {
		end = s.ended
	}
	return Status{ID: s.ID, Type: s.Q.Type, Pattern: s.Q.Pattern, Root: s.Q.Root, Complete: s.done, Total: len(s.hits),
		RuntimeMS: end.Sub(s.Started).Milliseconds(), Skipped: s.skipped, Error: s.err}
}

// Wait blocks until the session completes or d passes (lets start_search return first results at once).
func (s *Session) Wait(d time.Duration) {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if s.Status().Complete {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (m *Manager) reap() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.all {
		s.mu.Lock()
		old := s.done && time.Since(s.ended) > m.Keep
		s.mu.Unlock()
		if old {
			delete(m.all, id)
		}
	}
}
