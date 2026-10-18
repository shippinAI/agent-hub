package procs

import (
	"errors"
	"io"
	"os/exec"
	"sort"
	"sync"
	"time"
)

// handle is a started process on any OS (exec.Cmd, PTY, or ConPTY).
type handle interface {
	Pid() int
	Wait() (int, error)
	Kill() error // the whole tree
	Stdin() io.Writer
}

// Session is one running (or finished, not yet reaped) process.
type Session struct {
	PID     int
	Command string
	Started time.Time
	PTY     bool
	Lease   string
	Fence   int64
	TTL     time.Duration
	out     *Buffer
	h       handle
	done    chan struct{}
	mu      sync.Mutex
	exit    *int
	cursor  int // next unread absolute line (DC offset=0 reads)
	used    time.Time
	beat    time.Time
}

// Spec starts a session. Cmd is already confined by the sandbox package.
type Spec struct {
	Cmd     *exec.Cmd
	PTY     bool
	Command string // display label (the script or argv joined)
	Lease   string // hub lease id, if any
	Fence   int64
	TTL     time.Duration // hub lease TTL: no heartbeat within it = killed
}

// Manager owns all sessions of one server.
type Manager struct {
	mu      sync.Mutex
	byPID   map[int]*Session
	BufMax  int
	Idle    time.Duration
	Prompts Prompts
}

// NewManager makes a manager; call Reap periodically (Run does).
func NewManager(bufMax int, idle time.Duration, p Prompts) *Manager {
	return &Manager{byPID: map[int]*Session{}, BufMax: bufMax, Idle: idle, Prompts: p}
}

// Start launches the process and begins collecting stdout+stderr.
func (m *Manager) Start(s Spec) (*Session, error) {
	buf := NewBuffer(m.BufMax)
	h, err := start(s.Cmd, s.PTY, buf)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	ss := &Session{PID: h.Pid(), Command: s.Command, Started: now, PTY: s.PTY, Lease: s.Lease, Fence: s.Fence, TTL: s.TTL,
		out: buf, h: h, done: make(chan struct{}), used: now, beat: now}
	go func() {
		code, _ := h.Wait()
		ss.mu.Lock()
		ss.exit = &code
		ss.mu.Unlock()
		close(ss.done)
	}()
	m.mu.Lock()
	m.byPID[ss.PID] = ss
	m.mu.Unlock()
	return ss, nil
}

// Get finds a session by pid.
func (m *Manager) Get(pid int) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.byPID[pid]
	return s, ok
}

// ByLease finds a session by hub lease id.
func (m *Manager) ByLease(id string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.byPID {
		if s.Lease == id && id != "" {
			return s, true
		}
	}
	return nil, false
}

// Info is a list_sessions row.
type Info struct {
	PID       int    `json:"pid"`
	Command   string `json:"command"`
	State     string `json:"state"`
	Blocked   bool   `json:"blocked"`
	RuntimeMS int64  `json:"runtime_ms"`
	ExitCode  *int   `json:"exit_code,omitempty"`
	PTY       bool   `json:"pty"`
}

// List returns every session, oldest first.
func (m *Manager) List() []Info {
	m.mu.Lock()
	all := make([]*Session, 0, len(m.byPID))
	for _, s := range m.byPID {
		all = append(all, s)
	}
	m.mu.Unlock()
	sort.Slice(all, func(i, j int) bool { return all[i].Started.Before(all[j].Started) })
	out := make([]Info, 0, len(all))
	for _, s := range all {
		st := s.State(m.Prompts)
		out = append(out, Info{PID: s.PID, Command: s.Command, State: st, Blocked: st == StateWaiting,
			RuntimeMS: time.Since(s.Started).Milliseconds(), ExitCode: s.ExitCode(), PTY: s.PTY})
	}
	return out
}

// ErrNoSession is returned for an unknown pid.
var ErrNoSession = errors.New("no such session (it was never started here, or was reaped)")

// Terminate kills a session's whole tree and waits briefly for it to exit.
func (m *Manager) Terminate(pid int) error {
	s, ok := m.Get(pid)
	if !ok {
		return ErrNoSession
	}
	return s.Kill()
}
