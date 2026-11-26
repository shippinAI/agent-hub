// hub-seat is the factory's agent runtime: it claims ready tasks from the hub as one agent and runs a headless
// coding agent on each, confined to a git worktree. See internal/seat.
//
//	hub-seat --hub URL --token-file F --repo DIR --work-root DIR [flags]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"agenthub/internal/seat"
	"devagotchi/bridge/sandbox"
)

type multi []string

func (m *multi) String() string     { return strings.Join(*m, ",") }
func (m *multi) Set(v string) error { *m = append(*m, v); return nil }

func main() {
	if len(os.Args) > 1 && os.Args[1] == "sandbox-init" {
		if err := sandbox.Init(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(127)
		}
		return
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "hub-seat:", err)
		os.Exit(1)
	}
}

func readSecret(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	b, err := os.ReadFile(path)
	return strings.TrimSpace(string(b)), err
}

func run() error {
	var (
		hub, tokenFile, repo, repoName, base, workRoot, logDir, name = "", "", "", "", "", "", "", ""
		sandboxLevel, provider, billing, push, ghTokenFile, prBase   = "", "", "", "", "", ""
		escalate, project, agent                                     string
		envs, hosts, checks, labels                                  multi
		timeout, ciWait, idle                                        time.Duration
		once                                                         bool
	)
	fs := flag.NewFlagSet("hub-seat", flag.ExitOnError)
	fs.StringVar(&hub, "hub", os.Getenv("HUB_URL"), "hub base URL")
	fs.StringVar(&tokenFile, "token-file", os.Getenv("HUB_TOKEN_FILE"), "file holding this seat's agent token")
	fs.StringVar(&name, "name", "hub-seat", "seat name (commits, logs)")
	fs.StringVar(&repo, "repo", ".", "local git checkout the worktrees come from")
	fs.StringVar(&repoName, "repo-name", "", "GitHub owner/name, for git links and PRs")
	fs.StringVar(&base, "base", "HEAD", "ref new task branches start from")
	fs.StringVar(&workRoot, "work-root", "", "directory for worktrees (outside the repo)")
	fs.StringVar(&logDir, "log-dir", "", "transcript directory (default WORK_ROOT/logs)")
	fs.StringVar(&agent, "agent", "claude -p --output-format stream-json --verbose --dangerously-skip-permissions",
		"agent command (space-separated); the prompt goes to stdin")
	fs.Var(&envs, "env", "NAME to pass from this environment into the sandbox (repeatable), e.g. ANTHROPIC_API_KEY")
	fs.Var(&hosts, "allow-host", "egress host the agent may reach (repeatable; default api.anthropic.com)")
	fs.StringVar(&sandboxLevel, "sandbox", "workspace-write", "workspace-write or unsandboxed")
	fs.DurationVar(&timeout, "timeout", 60*time.Minute, "per-task agent time limit")
	fs.StringVar(&provider, "provider", "anthropic-api", "usage provider_id")
	fs.StringVar(&billing, "billing", "api_key", "api_key, subscription or router")
	fs.Var(&checks, "check", "KIND=COMMAND run in the worktree after the agent, recorded as evidence (repeatable)")
	fs.StringVar(&push, "push", "", "remote to push task branches to (empty = no push)")
	fs.StringVar(&ghTokenFile, "github-token-file", "", "token that can open PRs (with --push and --repo-name)")
	fs.StringVar(&prBase, "pr-base", "main", "branch PRs target")
	fs.DurationVar(&ciWait, "ci-wait", 0, "hold the lease this long waiting for required evidence (e.g. CI)")
	fs.StringVar(&escalate, "escalate-to", "", "principal that failed tasks are handed to")
	fs.StringVar(&project, "project", "", "only claim tasks in this project")
	fs.Var(&labels, "label", "only claim tasks with this label (repeatable)")
	fs.BoolVar(&once, "once", false, "work at most one task, then exit")
	fs.DurationVar(&idle, "idle", 30*time.Second, "wait between polls when nothing is ready")
	_ = fs.Parse(os.Args[1:])

	token, err := readSecret(tokenFile)
	if err != nil || token == "" || hub == "" {
		return fmt.Errorf("--hub and --token-file are required (%v)", err)
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	c := seat.Config{Name: name, Hub: &seat.Client{URL: hub, Token: token}, Repo: repo, RepoName: repoName, Base: base,
		WorkRoot: workRoot, LogDir: logDir, Agent: strings.Fields(agent), Env: map[string]string{}, Sandbox: sandboxLevel,
		Self: self, AllowHosts: hosts, Timeout: timeout, Provider: provider, BillingKind: billing, Push: push, CIWait: ciWait,
		EscalateTo: escalate, Project: project, Labels: labels,
		Log: func(s string) { fmt.Fprintln(os.Stderr, time.Now().Format(time.TimeOnly), s) }}
	if len(c.AllowHosts) == 0 {
		c.AllowHosts = []string{"api.anthropic.com"}
	}
	for _, k := range envs {
		if v, ok := os.LookupEnv(k); ok {
			c.Env[k] = v
		}
	}
	for _, ch := range checks {
		k, cmd, ok := strings.Cut(ch, "=")
		if !ok || k == "" || cmd == "" {
			return fmt.Errorf("--check wants KIND=COMMAND, got %q", ch)
		}
		c.Checks = append(c.Checks, seat.Check{Kind: k, Argv: []string{"sh", "-c", cmd}})
	}
	if ghTokenFile != "" {
		gh, err := readSecret(ghTokenFile)
		if err != nil {
			return err
		}
		c.PR = &seat.GitHubPR{Token: gh, Base: prBase}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if once {
		o, err := seat.RunOnce(ctx, c)
		fmt.Printf("task=%d state=%s branch=%s head=%s reason=%q\n", o.TaskID, o.State, o.Branch, o.Head, o.Reason)
		return err
	}
	return seat.Run(ctx, c, idle)
}
