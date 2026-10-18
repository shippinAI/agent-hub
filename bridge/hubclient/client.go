// Package hubclient talks to the hub's /bridge/v1 endpoints (long-poll and WebSocket).
package hubclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"devagotchi/bridge/wire"
)

// Client is one bridge's connection to its hub.
type Client struct {
	URL   string
	Token string
	HTTP  *http.Client
}

// New returns a client.
func New(url, token string) *Client {
	return &Client{URL: strings.TrimRight(url, "/"), Token: token, HTTP: &http.Client{Timeout: 70 * time.Second}}
}

func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.URL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if resp.StatusCode/100 != 2 {
		var e struct {
			Error struct{ Code, Message string } `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		return fmt.Errorf("hub %s %s: %d %s %s", method, path, resp.StatusCode, e.Error.Code, e.Error.Message)
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// Enrolled is the enroll response.
type Enrolled struct {
	MachineID    string `json:"machine_id"`
	Token        string `json:"token"`
	PollSeconds  int    `json:"bridge_poll_seconds"`
	CacheSeconds int    `json:"grant_cache_seconds"`
}

// Enroll trades a one-time code for the machine token.
func (c *Client) Enroll(ctx context.Context, code, os, arch, version, sandbox, display string) (Enrolled, error) {
	var out Enrolled
	err := c.do(ctx, "POST", "/bridge/v1/enroll", map[string]string{"code": code, "os": os, "arch": arch, "bridge_version": version,
		"sandbox_kind": sandbox, "display_server": display}, &out)
	return out, err
}

// Poll long-polls for frames.
func (c *Client) Poll(ctx context.Context) ([]wire.Frame, error) {
	var out []wire.Frame
	err := c.do(ctx, "POST", "/bridge/v1/poll", nil, &out)
	return out, err
}

// Send posts frames (ack, chunk, result, gui_stop).
func (c *Client) Send(ctx context.Context, frames ...wire.Frame) error {
	return c.do(ctx, "POST", "/bridge/v1/result", frames, nil)
}

// GrantLive asks the hub whether a grant is still live (the bridge's cache refetch).
func (c *Client) GrantLive(ctx context.Context, id string) (bool, int64, error) {
	var out struct {
		Live    bool  `json:"live"`
		Version int64 `json:"version"`
	}
	err := c.do(ctx, "GET", "/bridge/v1/grant/"+id, nil, &out)
	return out.Live, out.Version, err
}

// GUIReady reports per-permission GUI state.
func (c *Client) GUIReady(ctx context.Context, ready map[string]string, display string) error {
	return c.do(ctx, "POST", "/bridge/v1/gui-ready", map[string]any{"gui_ready": ready, "display_server": display}, nil)
}
