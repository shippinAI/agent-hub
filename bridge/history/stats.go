package history

import (
	"sort"
	"time"
)

// ToolStats counts one tool.
type ToolStats struct {
	Calls     int    `json:"calls"`
	Failures  int    `json:"failures"`
	TotalMS   int64  `json:"total_ms"`
	MaxMS     int64  `json:"max_ms"`
	LastError string `json:"last_error,omitempty"`
}

// Stats is the persisted usage summary.
type Stats struct {
	FirstUse time.Time             `json:"first_use"`
	LastUse  time.Time             `json:"last_use"`
	Calls    int                   `json:"calls"`
	Failures int                   `json:"failures"`
	Tools    map[string]*ToolStats `json:"tools"`
}

func (s *Stats) add(c Call) {
	if s.FirstUse.IsZero() {
		s.FirstUse = c.At
	}
	s.LastUse = c.At
	s.Calls++
	t := s.Tools[c.Tool]
	if t == nil {
		t = &ToolStats{}
		s.Tools[c.Tool] = t
	}
	t.Calls++
	t.TotalMS += c.DurationMS
	t.MaxMS = max(t.MaxMS, c.DurationMS)
	if !c.OK {
		s.Failures++
		t.Failures++
		t.LastError = c.Error
	}
}

// Summary is what get_usage_stats returns.
type Summary struct {
	Stats
	SuccessRate float64          `json:"success_rate"`
	Top         []string         `json:"most_used"`
	AvgMS       map[string]int64 `json:"avg_ms"`
}

// Summary computes rates and averages.
func (l *Log) Summary() Summary {
	if l == nil {
		return Summary{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	s := Summary{Stats: l.stats, AvgMS: map[string]int64{}}
	if s.Calls > 0 {
		s.SuccessRate = float64(s.Calls-s.Failures) / float64(s.Calls)
	}
	for name, t := range s.Tools {
		s.Top = append(s.Top, name)
		if t.Calls > 0 {
			s.AvgMS[name] = t.TotalMS / int64(t.Calls)
		}
	}
	sort.Slice(s.Top, func(i, j int) bool { return s.Tools[s.Top[i]].Calls > s.Tools[s.Top[j]].Calls })
	if len(s.Top) > 10 {
		s.Top = s.Top[:10]
	}
	return s
}
