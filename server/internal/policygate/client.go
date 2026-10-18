// Package policygate is the hub's client for the gold-policy decision point (POST <url>/decide).
// It fails closed: a timeout, a non-200, or an answer without "allow" is an error, and the hub blocks.
package policygate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"agenthub/internal/access"
	"agenthub/internal/store"
	"devagotchi/bridge/scope"
)

// Client talks to one PDP. Token is a bearer secret read from a file; it is never logged.
type Client struct {
	URL   string
	token string
	err   error
	HTTP  *http.Client
}

// FromEnv builds a client from HUB_POLICY_URL (e.g. http://127.0.0.1:8095/v1) and optional
// HUB_POLICY_TOKEN_FILE. Returns nil when no URL is set. An unreadable token file makes every call fail.
func FromEnv() *Client {
	u := strings.TrimRight(strings.TrimSpace(os.Getenv("HUB_POLICY_URL")), "/")
	if u == "" {
		return nil
	}
	c := New(u)
	if f := strings.TrimSpace(os.Getenv("HUB_POLICY_TOKEN_FILE")); f != "" {
		b, err := os.ReadFile(f)
		if err != nil || len(strings.TrimSpace(string(b))) == 0 {
			c.err = fmt.Errorf("HUB_POLICY_TOKEN_FILE unreadable or empty")
		}
		c.token = strings.TrimSpace(string(b))
	}
	return c
}

// New returns a client with a 3 s timeout (the PDP answers locally in milliseconds).
func New(url string) *Client {
	return &Client{URL: strings.TrimRight(url, "/"), HTTP: &http.Client{Timeout: 3 * time.Second}}
}

type answer struct {
	DecisionID string `json:"decision_id"`
	Allow      *bool  `json:"allow"`
	Reason     string `json:"reason"`
	Decision   struct {
		Version int64 `json:"policy_version"`
	} `json:"decision"`
}

// Decide asks the PDP about one bridge call. The machine id is sent as a host, so region globs
// (e.g. WI/TX/VA) match hub machines named after their site.
func (c *Client) Decide(ctx context.Context, p *store.Principal, machine string, call scope.Call) (access.PolicyVerdict, error) {
	principal := p.ID
	if c.err != nil {
		return access.PolicyVerdict{}, c.err
	}
	body, _ := json.Marshal(map[string]any{"event": "pre", "source": "agent-hub", "tool": "hub:" + call.Op, "argv": call.Argv,
		"paths": call.Paths, "hosts": append(append([]string{}, call.Hosts...), machine), "principal": principal})
	req, err := http.NewRequestWithContext(ctx, "POST", c.URL+"/decide", bytes.NewReader(body))
	if err != nil {
		return access.PolicyVerdict{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	r, err := c.HTTP.Do(req)
	if err != nil {
		return access.PolicyVerdict{}, errors.New("PDP unreachable")
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return access.PolicyVerdict{}, fmt.Errorf("PDP answered HTTP %d", r.StatusCode)
	}
	var a answer
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil || a.Allow == nil {
		return access.PolicyVerdict{}, errors.New("PDP answer malformed")
	}
	return access.PolicyVerdict{Allow: *a.Allow, Reason: a.Reason, DecisionID: a.DecisionID, Version: a.Decision.Version}, nil
}
