package seat

import (
	"strings"
	"testing"
)

func TestStreamParsesClaudeCodeOutput(t *testing.T) {
	lines := `{"type":"system","subtype":"init","session_id":"s1","model":"claude-test","tools":[]}
not json
{"type":"assistant","session_id":"s1","message":{"id":"m1","model":"claude-test","usage":{"input_tokens":10,"cache_read_input_tokens":90,"cache_creation_input_tokens":5,"output_tokens":20}}}
{"type":"assistant","session_id":"s1","message":{"id":"m1","model":"claude-test","usage":{"input_tokens":10,"cache_read_input_tokens":90,"cache_creation_input_tokens":5,"output_tokens":20}}}
{"type":"assistant","session_id":"s1","message":{"id":"m2","model":"claude-test","usage":{"input_tokens":1,"output_tokens":2}}}`
	var s Stream
	for _, l := range strings.Split(lines, "\n") {
		s.Add([]byte(l))
	}
	u := s.FinalUsage()
	if s.Done || s.Turns != 2 || len(u) != 1 || u[0].Input != 101 || u[0].CacheRead != 90 || u[0].CacheWrite != 5 || u[0].Output != 22 || u[0].CostMicros != nil {
		t.Fatalf("partial run: %+v %+v", s, u)
	}
	s.Add([]byte(`{"type":"result","subtype":"success","is_error":false,"result":"done","num_turns":3,"session_id":"s1","total_cost_usd":0.5,
		"usage":{"input_tokens":11,"output_tokens":22},
		"modelUsage":{"claude-test":{"inputTokens":11,"outputTokens":22,"cacheReadInputTokens":90,"cacheCreationInputTokens":5,"costUSD":0.45},
		"claude-small":{"inputTokens":3,"outputTokens":4,"costUSD":0.05}}}`))
	u = s.FinalUsage()
	if !s.Done || s.IsError || s.Result != "done" || s.Turns != 3 || len(u) != 2 {
		t.Fatalf("result: %+v", s)
	}
	if u[0].Model != "claude-small" || *u[0].CostMicros != 50_000 || u[1].Input != 101 || *u[1].CostMicros != 450_000 {
		t.Fatalf("model usage: %+v %+v", u[0], u[1])
	}
}
