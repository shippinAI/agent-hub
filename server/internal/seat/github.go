package seat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GitHubPR opens draft pull requests with a token that can write pull requests on the repo.
type GitHubPR struct {
	Token string
	API   string // default https://api.github.com
	Base  string // target branch, e.g. master
}

// Open creates a draft PR for branch, or returns the open one that already exists.
func (g *GitHubPR) Open(ctx context.Context, repo, branch, title, body string) (int, string, error) {
	api := strings.TrimRight(g.API, "/")
	if api == "" {
		api = "https://api.github.com"
	}
	owner := strings.SplitN(repo, "/", 2)[0]
	var pr struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
	}
	in := map[string]any{"title": title, "head": branch, "base": g.Base, "body": body, "draft": true}
	code, err := g.call(ctx, "POST", api+"/repos/"+repo+"/pulls", in, &pr)
	if err == nil {
		return pr.Number, pr.HTMLURL, nil
	}
	if code != http.StatusUnprocessableEntity {
		return 0, "", err
	}
	var open []struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
	}
	q := url.Values{"head": {owner + ":" + branch}, "state": {"open"}}
	if _, err := g.call(ctx, "GET", api+"/repos/"+repo+"/pulls?"+q.Encode(), nil, &open); err != nil || len(open) == 0 {
		return 0, "", fmt.Errorf("create PR refused and no open PR found for %s", branch)
	}
	return open[0].Number, open[0].HTMLURL, nil
}

func (g *GitHubPR) call(ctx context.Context, method, u string, body, out any) (int, error) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+g.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("github %s %d: %s", method, resp.StatusCode, firstLine(string(raw), 300))
	}
	return resp.StatusCode, json.Unmarshal(raw, out)
}
