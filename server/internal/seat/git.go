package seat

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// git runs one git command in dir and returns trimmed stdout.
func git(ctx context.Context, dir string, args ...string) (string, error) {
	c := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	c.Env = append(c.Environ(), "GIT_TERMINAL_PROMPT=0")
	var out, errb bytes.Buffer
	c.Stdout, c.Stderr = &out, &errb
	if err := c.Run(); err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// branchName is hub/<id>-<slug>: stable per task, so a later lease continues the same branch.
func branchName(id int64, title string) string {
	s := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(title), "-"), "-")
	if len(s) > 40 {
		s = strings.TrimRight(s[:40], "-")
	}
	if s == "" {
		return fmt.Sprintf("hub/%d", id)
	}
	return fmt.Sprintf("hub/%d-%s", id, s)
}

// addWorktree checks the task's branch out at dir: the existing branch when an earlier lease left one,
// otherwise a new branch from base.
func addWorktree(ctx context.Context, repo, dir, branch, base string) error {
	if _, err := git(ctx, repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
		_, err = git(ctx, repo, "worktree", "add", dir, branch)
		return err
	}
	_, err := git(ctx, repo, "worktree", "add", "-b", branch, dir, base)
	return err
}

func removeWorktree(repo, dir string) {
	_, _ = git(context.Background(), repo, "worktree", "remove", "--force", dir)
	_, _ = git(context.Background(), repo, "worktree", "prune")
}

// commitAll commits every change in the worktree as the seat and returns the new head.
func commitAll(ctx context.Context, dir, seat, msg string) (bool, error) {
	if _, err := git(ctx, dir, "add", "-A"); err != nil {
		return false, err
	}
	st, err := git(ctx, dir, "status", "--porcelain")
	if err != nil || st == "" {
		return false, err
	}
	_, err = git(ctx, dir, "-c", "user.name="+seat, "-c", "user.email="+seat+"@agent-hub.invalid", "-c", "commit.gpgsign=false",
		"commit", "-q", "-m", msg)
	return err == nil, err
}
