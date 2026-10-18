// Package history is the local-only record behind get_usage_stats and get_recent_tool_calls. It lives in
// the config dir (0600), is redacted before it is written, is bounded, and is never sent anywhere.
package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"devagotchi/bridge/redact"
)

// Call is one recorded tool call.
type Call struct {
	At         time.Time       `json:"at"`
	Tool       string          `json:"tool"`
	Args       json.RawMessage `json:"args,omitempty"`
	OK         bool            `json:"ok"`
	Error      string          `json:"error,omitempty"`
	DurationMS int64           `json:"duration_ms"`
	Output     string          `json:"output,omitempty"`
	Client     string          `json:"client,omitempty"`
}

// Log is the history store. A nil *Log or Enabled=false records nothing.
type Log struct {
	mu      sync.Mutex
	dir     string
	max     int
	outCap  int
	red     redact.Redactor
	recent  []Call
	stats   Stats
	Enabled bool
}

// Open loads the history and stats in dir. maxEntries bounds the file; outCap bounds stored output.
func Open(dir string, enabled bool, maxEntries, outCap int, red redact.Redactor) *Log {
	l := &Log{dir: dir, max: maxEntries, outCap: outCap, red: red, Enabled: enabled, stats: Stats{Tools: map[string]*ToolStats{}}}
	if !enabled || dir == "" {
		return l
	}
	if f, err := os.Open(filepath.Join(dir, "history.jsonl")); err == nil {
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 8<<20)
		for sc.Scan() {
			var c Call
			if json.Unmarshal(sc.Bytes(), &c) == nil {
				l.recent = append(l.recent, c)
			}
		}
		f.Close()
		if len(l.recent) > l.max {
			l.recent = l.recent[len(l.recent)-l.max:]
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, "usage.json")); err == nil {
		_ = json.Unmarshal(b, &l.stats)
		if l.stats.Tools == nil {
			l.stats.Tools = map[string]*ToolStats{}
		}
	}
	return l
}

// Record stores one call (args and output redacted and capped).
func (l *Log) Record(c Call) {
	if l == nil || !l.Enabled {
		return
	}
	c.Args = json.RawMessage(l.red.String(string(capArgs(c.Args))))
	if len(c.Output) > l.outCap {
		c.Output = c.Output[:l.outCap] + "…[output omitted]"
	}
	c.Output = l.red.String(c.Output)
	l.mu.Lock()
	defer l.mu.Unlock()
	l.recent = append(l.recent, c)
	l.stats.add(c)
	if l.dir == "" {
		return
	}
	_ = os.MkdirAll(l.dir, 0o700)
	if len(l.recent) > 2*l.max {
		l.recent = l.recent[len(l.recent)-l.max:]
		l.rewrite()
	} else if f, err := os.OpenFile(filepath.Join(l.dir, "history.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
		b, _ := json.Marshal(c)
		_, _ = f.Write(append(b, '\n'))
		f.Close()
	}
	if b, err := json.Marshal(l.stats); err == nil {
		_ = os.WriteFile(filepath.Join(l.dir, "usage.json"), b, 0o600)
	}
}

func (l *Log) rewrite() {
	tmp := filepath.Join(l.dir, ".history.jsonl.tmp")
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	for _, c := range l.recent {
		b, _ := json.Marshal(c)
		_, _ = f.Write(append(b, '\n'))
	}
	f.Close()
	_ = os.Rename(tmp, filepath.Join(l.dir, "history.jsonl"))
}

// Recent returns up to n calls, newest last, filtered by tool and time.
func (l *Log) Recent(n int, tool string, since time.Time) []Call {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []Call
	for i := len(l.recent) - 1; i >= 0 && len(out) < n; i-- {
		c := l.recent[i]
		if (tool == "" || c.Tool == tool) && !c.At.Before(since) {
			out = append([]Call{c}, out...)
		}
	}
	return out
}

func capArgs(raw json.RawMessage) json.RawMessage {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return raw
	}
	for k, v := range m {
		if s, ok := v.(string); ok && len(s) > 512 {
			m[k] = s[:512] + "…"
		}
	}
	b, _ := json.Marshal(m)
	return b
}
