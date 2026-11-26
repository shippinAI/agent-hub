package seat

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is the seat's view of the hub REST API, authenticated as one agent principal.
type Client struct {
	URL   string // hub base URL, e.g. http://127.0.0.1:8080
	Token string
	HTTP  *http.Client
}

// APIError is a hub error response.
type APIError struct {
	Status  int
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Current map[string]any `json:"current"`
}

func (e *APIError) Error() string { return fmt.Sprintf("hub %d %s: %s", e.Status, e.Code, e.Message) }

// IsCode reports whether err is a hub error with that code.
func IsCode(err error, code string) bool {
	e, ok := err.(*APIError)
	return ok && e.Code == code
}

func newRequestID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return "seat-" + hex.EncodeToString(b)
}

// do sends one request. reqID makes a write idempotent: the same id is reused on network retries.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	reqID := newRequestID()
	var last error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt*attempt) * 500 * time.Millisecond):
			}
		}
		var rd io.Reader
		if body != nil {
			b, err := json.Marshal(body)
			if err != nil {
				return err
			}
			rd = bytes.NewReader(b)
		}
		req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.URL, "/")+"/api/v1"+path, rd)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.Token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", reqID)
		hc := c.HTTP
		if hc == nil {
			hc = &http.Client{Timeout: 60 * time.Second}
		}
		resp, err := hc.Do(req)
		if err != nil {
			last = err
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if resp.StatusCode >= 500 {
			last = fmt.Errorf("hub %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
			continue
		}
		if resp.StatusCode >= 300 {
			var env struct {
				Error APIError `json:"error"`
			}
			_ = json.Unmarshal(raw, &env)
			env.Error.Status = resp.StatusCode
			if env.Error.Code == "" {
				env.Error.Code, env.Error.Message = "http_error", strings.TrimSpace(string(raw))
			}
			return &env.Error
		}
		if out == nil {
			return nil
		}
		return json.Unmarshal(raw, out)
	}
	return last
}
