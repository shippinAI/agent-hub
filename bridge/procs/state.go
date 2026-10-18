package procs

import (
	"context"
	"io"
	"strings"
	"time"
)

// Session states.
const (
	StateRunning  = "running"
	StateWaiting  = "waiting_for_input"
	StateFinished = "finished"
)

// ExitCode is nil while running.
func (s *Session) ExitCode() *int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.exit
}

// State is finished, waiting_for_input (prompt at the end of output) or running.
func (s *Session) State(p Prompts) string {
	if s.ExitCode() != nil {
		return StateFinished
	}
	if p.Match(s.out.TailLine()) {
		return StateWaiting
	}
	return StateRunning
}

// Output is the session's buffer.
func (s *Session) Output() *Buffer { return s.out }

// Write sends input to stdin (adds a newline unless raw).
func (s *Session) Write(data string, raw bool) error {
	if s.ExitCode() != nil {
		return io.ErrClosedPipe
	}
	if !raw && !strings.HasSuffix(data, "\n") {
		data += "\n"
	}
	s.touch()
	_, err := io.WriteString(s.h.Stdin(), data)
	return err
}

// Kill ends the tree and waits up to 2 s for the exit to be observed.
func (s *Session) Kill() error {
	err := s.h.Kill()
	select {
	case <-s.done:
	case <-time.After(2 * time.Second):
	}
	return err
}

// Heartbeat extends a hub lease.
func (s *Session) Heartbeat() { s.mu.Lock(); s.beat = time.Now(); s.mu.Unlock() }

func (s *Session) touch() { s.mu.Lock(); s.used = time.Now(); s.mu.Unlock() }

// Snapshot is the outcome of a wait.
type Snapshot struct {
	State    string `json:"state"`
	Reason   string `json:"reason"` // exit | prompt | output | timeout | cancelled
	Output   string `json:"-"`
	Seq      int64  `json:"seq"`
	Lost     bool   `json:"output_dropped,omitempty"`
	ExitCode *int   `json:"exit_code,omitempty"`
	WaitedMS int64  `json:"waited_ms"`
}

// Wait collects output after seq until the process exits, a prompt shows (stopOnPrompt), any output
// arrives and settles (stopOnOutput), the timeout passes, or ctx ends. The process keeps running after a timeout.
func (s *Session) Wait(ctx context.Context, from int64, timeout time.Duration, p Prompts, stopOnPrompt, stopOnOutput bool) Snapshot {
	s.touch()
	start := time.Now()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	reason := ""
	for reason == "" {
		changed := s.out.Changed()
		switch {
		case s.ExitCode() != nil:
			reason = "exit"
		case stopOnPrompt && s.out.Seq() > from && p.Match(s.out.TailLine()):
			reason = "prompt"
		case stopOnOutput && s.out.Seq() > from && settle(s.out, 60*time.Millisecond):
			reason = "output"
		default:
			select {
			case <-changed:
			case <-s.done:
			case <-timer.C:
				reason = "timeout"
			case <-ctx.Done():
				reason = "cancelled"
			}
		}
	}
	out, seq, lost := s.out.Since(from)
	return Snapshot{State: s.State(p), Reason: reason, Output: out, Seq: seq, Lost: lost, ExitCode: s.ExitCode(), WaitedMS: time.Since(start).Milliseconds()}
}

// settle waits until no new output arrives for d (so a burst is returned whole).
func settle(b *Buffer, d time.Duration) bool {
	for {
		select {
		case <-b.Changed():
		case <-time.After(d):
			return true
		}
	}
}

// Done is closed when the process exits.
func (s *Session) Done() <-chan struct{} { return s.done }

// MarkRead moves the unread cursor to the end of the output.
func (s *Session) MarkRead() {
	lines, first := s.out.Lines()
	s.mu.Lock()
	s.cursor = first + len(lines)
	if n := len(lines); n > 0 && !strings.HasSuffix(lines[n-1], "\n") {
		s.cursor--
	}
	s.mu.Unlock()
}
