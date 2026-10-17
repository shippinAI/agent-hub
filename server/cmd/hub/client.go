package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type client struct {
	url, token, reqID string
	http              *http.Client
}

func newClient(reqID string) (*client, error) {
	url := strings.TrimRight(os.Getenv("HUB_URL"), "/")
	if url == "" {
		url = "http://127.0.0.1:8088"
	}
	tok := os.Getenv("HUB_TOKEN")
	if f := os.Getenv("HUB_TOKEN_FILE"); tok == "" && f != "" {
		b, err := os.ReadFile(expand(f))
		if err != nil {
			return nil, fmt.Errorf("read HUB_TOKEN_FILE: %w", err)
		}
		tok = strings.TrimSpace(string(b))
	}
	if tok == "" {
		return nil, fmt.Errorf("set HUB_TOKEN or HUB_TOKEN_FILE")
	}
	if reqID == "" {
		b := make([]byte, 12)
		_, _ = rand.Read(b)
		reqID = "cli-" + hex.EncodeToString(b)
	}
	return &client{url: url, token: tok, reqID: reqID, http: &http.Client{Timeout: 40 * time.Second}}, nil
}

// apiError carries the hub's error code so callers can react (e.g. drop a lost claim).
type apiError struct {
	Status  int
	Code    string
	Message string
}

func (e *apiError) Error() string { return fmt.Sprintf("%s (%d): %s", e.Code, e.Status, e.Message) }

func (c *client) do(method, path string, body any, out any) error {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.url+"/api/v1"+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	if method != "GET" {
		req.Header.Set("Idempotency-Key", c.reqID)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		var e struct {
			Error struct{ Code, Message string } `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		return &apiError{Status: resp.StatusCode, Code: e.Error.Code, Message: e.Error.Message}
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

func expand(p string) string {
	if strings.HasPrefix(p, "~/") {
		h, _ := os.UserHomeDir()
		return h + p[1:]
	}
	return p
}
