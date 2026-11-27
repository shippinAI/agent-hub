package store

import (
	"context"
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"time"
)

// LinkKinds are the git objects a task can point at.
var LinkKinds = []string{"branch", "commit", "pr", "check_run"}

var (
	repoName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}/[A-Za-z0-9_.-]{1,100}$`)
	gitSHA   = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

// TaskLink ties a task to a branch, commit, pull request or check run. Ref is the branch name, commit sha,
// PR number or check name; HeadSHA is the newest commit seen for a branch or PR, or the commit a check ran on.
type TaskLink struct {
	ID        int64           `json:"id"`
	TaskID    int64           `json:"task_id"`
	Kind      string          `json:"kind"`
	Repo      string          `json:"repo"`
	Ref       string          `json:"ref"`
	URL       string          `json:"url"`
	State     string          `json:"state"`
	HeadSHA   string          `json:"head_sha"`
	Data      json.RawMessage `json:"data"`
	Author    string          `json:"author"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// LinkInput is link_git's input. A supplied fence must be current.
type LinkInput struct {
	Kind    string `json:"kind"`
	Repo    string `json:"repo"`
	Ref     string `json:"ref"`
	URL     string `json:"url,omitempty"`
	HeadSHA string `json:"head_sha,omitempty"`
	Fence   *int64 `json:"fence,omitempty"`
}

const linkCols = `id,task_id,kind,repo,ref,url,state,head_sha,data,author,created_at,updated_at`

func scanLink(r interface{ Scan(...any) error }) (TaskLink, error) {
	var l TaskLink
	err := r.Scan(&l.ID, &l.TaskID, &l.Kind, &l.Repo, &l.Ref, &l.URL, &l.State, &l.HeadSHA, &l.Data, &l.Author, &l.CreatedAt, &l.UpdatedAt)
	return l, err
}

func (l *TaskLink) clean() error {
	l.Kind, l.Repo, l.Ref = strings.TrimSpace(l.Kind), strings.TrimSpace(l.Repo), strings.TrimSpace(l.Ref)
	l.HeadSHA = strings.ToLower(strings.TrimSpace(l.HeadSHA))
	switch {
	case !slices.Contains(LinkKinds, l.Kind):
		return errInvalid("link kind must be one of %v", LinkKinds)
	case !repoName.MatchString(l.Repo):
		return errInvalid("repo must be owner/name")
	case l.Ref == "" || len(l.Ref) > 255:
		return errInvalid("ref must be 1-255 chars")
	case len(l.URL) > 2000:
		return errInvalid("url is limited to 2000 chars")
	case l.URL != "" && !strings.HasPrefix(l.URL, "https://"):
		return errInvalid("url must start with https://")
	case l.HeadSHA != "" && !gitSHA.MatchString(l.HeadSHA):
		return errInvalid("head_sha must be a 40-character commit sha")
	case l.Kind == "commit" && !gitSHA.MatchString(strings.ToLower(l.Ref)):
		return errInvalid("a commit link's ref is its 40-character sha")
	}
	if l.Kind == "commit" {
		l.Ref = strings.ToLower(l.Ref)
	}
	if len(l.Data) == 0 {
		l.Data = json.RawMessage("{}")
	}
	return nil
}

// link inserts or refreshes one link (keyed by task, kind, repo, ref). Empty fields never overwrite known ones.
// It logs git.linked for a new link and git.updated when the state or head moved.
func (w *W) link(t Task, l TaskLink) (TaskLink, error) {
	if err := l.clean(); err != nil {
		return l, err
	}
	if err := w.can("task.link", resOf(t)); err != nil {
		return l, err
	}
	var prev *TaskLink
	if old, err := scanLink(w.row(`SELECT `+linkCols+` FROM task_links WHERE task_id=$1 AND kind=$2 AND repo=$3 AND ref=$4 FOR UPDATE`,
		t.ID, l.Kind, l.Repo, l.Ref)); err == nil {
		prev = &old
	}
	out, err := scanLink(w.row(`INSERT INTO task_links(task_id,kind,repo,ref,url,state,head_sha,data,author)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (task_id,kind,repo,ref) DO UPDATE SET
			url=CASE WHEN excluded.url<>'' THEN excluded.url ELSE task_links.url END,
			state=CASE WHEN excluded.state<>'' THEN excluded.state ELSE task_links.state END,
			head_sha=CASE WHEN excluded.head_sha<>'' THEN excluded.head_sha ELSE task_links.head_sha END,
			data=task_links.data || excluded.data, updated_at=clock_timestamp()
		RETURNING `+linkCols, t.ID, l.Kind, l.Repo, l.Ref, l.URL, l.State, l.HeadSHA, []byte(l.Data), w.P.ID))
	if err != nil {
		return out, err
	}
	ev := map[string]any{"link_id": out.ID, "kind": out.Kind, "repo": out.Repo, "ref": out.Ref, "state": out.State, "head_sha": out.HeadSHA}
	switch {
	case prev == nil:
		return out, w.Event("git.linked", &t.ID, nil, ev)
	case prev.State != out.State || prev.HeadSHA != out.HeadSHA:
		ev["previous_state"], ev["previous_head_sha"] = prev.State, prev.HeadSHA
		return out, w.Event("git.updated", &t.ID, nil, ev)
	}
	return out, nil
}

// LinkGit records that a task's work lives on a branch, commit, PR or check run.
func (s *Store) LinkGit(ctx context.Context, p *Principal, reqID string, taskID int64, in LinkInput) (any, error) {
	return s.write(ctx, p, reqID, "link_git", []any{taskID, in}, func(w *W) (any, error) {
		t, err := getTask(ctx, w.tx, taskID, true)
		if err != nil {
			return nil, err
		}
		if in.Fence != nil && *in.Fence != t.Fence {
			e := fail(409, "stale_fence", "fence %d is not current", *in.Fence)
			e.Current = map[string]any{"fence": t.Fence, "holder": t.Holder}
			return nil, e
		}
		return w.link(t, TaskLink{Kind: in.Kind, Repo: in.Repo, Ref: in.Ref, URL: in.URL, HeadSHA: in.HeadSHA})
	})
}

// Links lists a task's git links, oldest first.
func (s *Store) Links(ctx context.Context, taskID int64) ([]TaskLink, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+linkCols+` FROM task_links WHERE task_id=$1 ORDER BY id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TaskLink{}
	for rows.Next() {
		l, err := scanLink(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
