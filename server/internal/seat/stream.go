package seat

import (
	"encoding/json"
	"math"
	"sort"
)

// Stream reads Claude Code's `--output-format stream-json` lines. It keeps what the seat needs: the session,
// the model, the turn count, the final result and token usage per model.
type Stream struct {
	SessionID string
	Model     string
	Turns     int
	Done      bool    // a result line arrived
	IsError   bool    // the result says the run failed
	Subtype   string  // success | error_max_turns | error_during_execution | ...
	Result    string  // final text
	CostUSD   float64 // the client's own estimate for the whole run
	Usage     []ModelUsage
	seenMsgs  map[string]bool
	sumByMsgs ModelUsage
}

// ModelUsage is one model's tokens in the hub's terms: CacheRead is part of Input, CacheWrite is not.
type ModelUsage struct {
	Model      string
	Input      int64
	CacheRead  int64
	CacheWrite int64
	Output     int64
	CostMicros *int64 // nil = not reported
}

type anthropicUsage struct {
	Input      int64 `json:"input_tokens"`
	CacheWrite int64 `json:"cache_creation_input_tokens"`
	CacheRead  int64 `json:"cache_read_input_tokens"`
	Output     int64 `json:"output_tokens"`
}

type modelUsage struct {
	Input      int64    `json:"inputTokens"`
	Output     int64    `json:"outputTokens"`
	CacheRead  int64    `json:"cacheReadInputTokens"`
	CacheWrite int64    `json:"cacheCreationInputTokens"`
	CostUSD    *float64 `json:"costUSD"`
}

type streamLine struct {
	Type       string                `json:"type"`
	Subtype    string                `json:"subtype"`
	SessionID  string                `json:"session_id"`
	Model      string                `json:"model"`
	IsError    bool                  `json:"is_error"`
	Result     string                `json:"result"`
	NumTurns   int                   `json:"num_turns"`
	TotalCost  *float64              `json:"total_cost_usd"`
	Usage      *anthropicUsage       `json:"usage"`
	ModelUsage map[string]modelUsage `json:"modelUsage"`
	Message    *struct {
		ID    string          `json:"id"`
		Model string          `json:"model"`
		Usage *anthropicUsage `json:"usage"`
	} `json:"message"`
}

func micros(usd float64) *int64 { v := int64(math.Round(usd * 1e6)); return &v }

// Add parses one line; lines that are not JSON or not understood are ignored.
func (s *Stream) Add(line []byte) {
	var l streamLine
	if json.Unmarshal(line, &l) != nil {
		return
	}
	if l.SessionID != "" {
		s.SessionID = l.SessionID
	}
	switch l.Type {
	case "system":
		if l.Subtype == "init" && l.Model != "" {
			s.Model = l.Model
		}
	case "assistant":
		if l.Message == nil || l.Message.ID == "" {
			return
		}
		if s.seenMsgs == nil {
			s.seenMsgs = map[string]bool{}
		}
		if s.seenMsgs[l.Message.ID] { // one API message is streamed as several lines carrying the same usage
			return
		}
		s.seenMsgs[l.Message.ID] = true
		s.Turns++
		if l.Message.Model != "" && s.Model == "" {
			s.Model = l.Message.Model
		}
		if u := l.Message.Usage; u != nil {
			s.sumByMsgs.Input += u.Input + u.CacheRead
			s.sumByMsgs.CacheRead += u.CacheRead
			s.sumByMsgs.CacheWrite += u.CacheWrite
			s.sumByMsgs.Output += u.Output
		}
	case "result":
		s.Done, s.IsError, s.Subtype, s.Result = true, l.IsError, l.Subtype, l.Result
		if l.NumTurns > 0 {
			s.Turns = l.NumTurns
		}
		if l.TotalCost != nil {
			s.CostUSD = *l.TotalCost
		}
		s.Usage = nil
		if len(l.ModelUsage) > 0 {
			names := make([]string, 0, len(l.ModelUsage))
			for m := range l.ModelUsage {
				names = append(names, m)
			}
			sort.Strings(names)
			for _, m := range names {
				u := l.ModelUsage[m]
				mu := ModelUsage{Model: m, Input: u.Input + u.CacheRead, CacheRead: u.CacheRead, CacheWrite: u.CacheWrite, Output: u.Output}
				if u.CostUSD != nil {
					mu.CostMicros = micros(*u.CostUSD)
				}
				s.Usage = append(s.Usage, mu)
			}
		} else if u := l.Usage; u != nil {
			mu := ModelUsage{Model: s.Model, Input: u.Input + u.CacheRead, CacheRead: u.CacheRead, CacheWrite: u.CacheWrite, Output: u.Output}
			if l.TotalCost != nil {
				mu.CostMicros = micros(*l.TotalCost)
			}
			s.Usage = []ModelUsage{mu}
		}
	}
}

// FinalUsage is the usage to record: the result's per-model figures, or, when the run died before its result
// line, the tokens summed from the assistant messages seen (cost unknown).
func (s *Stream) FinalUsage() []ModelUsage {
	if s.Done {
		return s.Usage
	}
	if s.sumByMsgs == (ModelUsage{}) {
		return nil
	}
	u := s.sumByMsgs
	u.Model = s.Model
	return []ModelUsage{u}
}
