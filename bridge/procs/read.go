package procs

import (
	"strings"
	"time"
)

// LineRead is read_process_output's window (DC semantics: offset 0 = new since last read, >0 absolute, <0 tail).
type LineRead struct {
	Text      string `json:"-"`
	From      int    `json:"from_line"`
	Returned  int    `json:"lines_returned"`
	Total     int    `json:"total_lines"`
	Remaining int    `json:"remaining_lines"`
	Dropped   int    `json:"lines_dropped_from_buffer,omitempty"`
}

// ReadLines returns a window of output lines and advances the unread cursor for offset 0.
func (s *Session) ReadLines(offset, length int) LineRead {
	s.touch()
	lines, first := s.out.Lines()
	total := first + len(lines)
	s.mu.Lock()
	defer s.mu.Unlock()
	from := offset
	switch {
	case offset == 0:
		from = s.cursor
	case offset < 0:
		from = max(first, total+offset)
	}
	from = max(from, first)
	end := min(total, from+length)
	if from > end {
		from = end
	}
	r := LineRead{From: from, Total: total, Dropped: first}
	if from < end {
		r.Text = strings.Join(lines[from-first:end-first], "")
		r.Returned = end - from
	}
	r.Remaining = total - end
	if offset == 0 {
		// A trailing partial line (a prompt) is shown but stays unread, so it shows again until completed.
		s.cursor = end
		if end == total && end > from && !strings.HasSuffix(lines[end-first-1], "\n") {
			s.cursor = end - 1
		}
	}
	return r
}

// Reap kills sessions whose hub lease missed its heartbeat or that sat idle too long, and forgets finished
// sessions after the idle window. It returns the pids it killed.
func (m *Manager) Reap(now time.Time) []int {
	m.mu.Lock()
	all := make([]*Session, 0, len(m.byPID))
	for _, s := range m.byPID {
		all = append(all, s)
	}
	m.mu.Unlock()
	var killed []int
	for _, s := range all {
		s.mu.Lock()
		used, beat, done := s.used, s.beat, s.exit != nil
		s.mu.Unlock()
		switch {
		case done && now.Sub(used) > m.Idle:
			m.mu.Lock()
			delete(m.byPID, s.PID)
			m.mu.Unlock()
		case !done && s.TTL > 0 && now.Sub(beat) > s.TTL, !done && m.Idle > 0 && now.Sub(used) > m.Idle:
			_ = s.Kill()
			killed = append(killed, s.PID)
		}
	}
	return killed
}

// Run reaps every interval until stop is closed, then kills everything left.
func (m *Manager) Run(interval time.Duration, stop <-chan struct{}) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			m.KillAll()
			return
		case now := <-t.C:
			m.Reap(now)
		}
	}
}

// KillAll ends every running session (server shutdown: no orphans).
func (m *Manager) KillAll() {
	m.mu.Lock()
	all := make([]*Session, 0, len(m.byPID))
	for _, s := range m.byPID {
		all = append(all, s)
	}
	m.mu.Unlock()
	for _, s := range all {
		if s.ExitCode() == nil {
			_ = s.h.Kill()
		}
	}
}
