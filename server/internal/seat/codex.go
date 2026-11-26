package seat

import "encoding/json"

// Codex `codex exec --json` events (codex-rs/exec/src/exec_events.rs). The stream names no model, so the
// seat's configured model is used. cached_input_tokens is part of input_tokens and reasoning_output_tokens part
// of output_tokens, which is how the hub counts them.
type codexLine struct {
	Type     string `json:"type"`
	ThreadID string `json:"thread_id"`
	Message  string `json:"message"`
	Usage    *struct {
		Input      int64 `json:"input_tokens"`
		CacheRead  int64 `json:"cached_input_tokens"`
		CacheWrite int64 `json:"cache_write_input_tokens"`
		Output     int64 `json:"output_tokens"`
		Reasoning  int64 `json:"reasoning_output_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
	Item *struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"item"`
}

func (s *Stream) addCodex(line []byte) {
	var l codexLine
	if json.Unmarshal(line, &l) != nil {
		return
	}
	switch l.Type {
	case "thread.started":
		s.SessionID = l.ThreadID
	case "item.completed":
		if l.Item == nil {
			return
		}
		switch l.Item.Type {
		case "agent_message":
			s.Result = l.Item.Text
			s.Turns++
		case "command_execution", "file_change", "mcp_tool_call", "web_search":
			s.Turns++ // Codex has no per-message usage; a step is the closest thing to Claude's turn
		}
	case "turn.completed":
		s.Done = true
		if s.Subtype == "" {
			s.Subtype = "success"
		}
		if u := l.Usage; u != nil {
			if len(s.Usage) == 0 {
				s.Usage = []ModelUsage{{Model: s.Model}}
			}
			mu := &s.Usage[0]
			mu.Input += u.Input
			mu.CacheRead += u.CacheRead
			mu.CacheWrite += u.CacheWrite
			mu.Output += u.Output
			mu.Reasoning += u.Reasoning
		}
	case "turn.failed":
		s.Done, s.IsError, s.Subtype = true, true, "turn_failed"
		if l.Error != nil {
			s.Result = l.Error.Message
		}
	case "error":
		s.Done, s.IsError, s.Subtype, s.Result = true, true, "error", l.Message
	}
}
