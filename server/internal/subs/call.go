package subs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"agenthub/internal/providers"
)

// call sends the smallest request the target's wire shape allows and returns "" on a 2xx, else a short
// error. The key goes only in the auth header; callers redact it from the returned text.
func (s *Service) call(ctx context.Context, t providers.Target, model, key string) string {
	ping := []map[string]string{{"role": "user", "content": "ping"}}
	var body any = map[string]any{"model": model, "max_tokens": 1, "messages": ping}
	if t.Shape == "responses" {
		body = map[string]any{"model": model, "max_output_tokens": 16, "input": "ping"}
	}
	b, _ := json.Marshal(body)
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "POST", t.URL, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if t.Shape == "messages" {
		req.Header.Set("x-api-key", key)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	d := s.HTTP
	if d == nil {
		d = http.DefaultClient
	}
	resp, err := d.Do(req)
	if err != nil {
		return "provider unreachable: " + err.Error()
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(io.LimitReader(resp.Body, 400))
	if resp.StatusCode/100 != 2 {
		return fmt.Sprintf("%s said %d: %s", t.Router, resp.StatusCode, strings.TrimSpace(string(out)))
	}
	return ""
}
