// Package seat is the factory's agent runtime: it claims ready tasks as one agent principal and runs a
// headless coding agent (Claude Code in stream-json mode) inside the bridge sandbox on a git worktree. While the
// agent works it heartbeats the lease; afterwards it commits, links the branch, records token usage, runs the
// configured checks as evidence, optionally pushes and opens a draft PR, and completes or hands the task off.
package seat

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"devagotchi/bridge/netproxy"
	"devagotchi/bridge/sandbox"
)

// Check is one local verification run in the worktree after the agent; its result is evidence of Kind.
type Check struct {
	Kind string   // evidence kind, e.g. test.go
	Argv []string // run in the worktree, same sandbox, no network
}

// Config is one seat.
type Config struct {
	Name     string // shown in commits and logs (the principal is whoever owns the token)
	Hub      *Client
	Repo     string // local checkout the worktrees come from
	RepoName string // owner/name for git links and PRs ("" = no links)
	Base     string // ref new branches start from (default HEAD)
	WorkRoot string // where worktrees go (must be outside Repo)
	LogDir   string // transcripts (<task>-<fence>.jsonl); "" = WorkRoot/logs

	Agent       []string          // argv; the prompt goes to stdin
	Env         map[string]string // passed into the sandbox (e.g. ANTHROPIC_API_KEY)
	Sandbox     string            // workspace-write (default) | unsandboxed
	Self        string            // binary that handles `sandbox-init` (the seat binary)
	AllowHosts  []string          // egress allowlist for the agent (none = no network)
	Timeout     time.Duration     // per agent run (default 60m)
	Provider    string            // usage provider_id (default anthropic-api)
	BillingKind string            // api_key (default) | subscription | router

	Checks      []Check
	Push        string        // remote to push the branch to ("" = no push)
	PR          *GitHubPR     // open a draft PR after pushing (nil = no)
	CIWait      time.Duration // keep the lease and retry complete while evidence is missing (0 = hand off at once)
	CIPoll      time.Duration // default 15s
	EscalateTo  string        // failed runs are handed to this principal so no seat re-claims them in a loop
	Project     string        // claim_next filter
	Labels      []string      // claim_next filter
	MaxFailures int           // Run stops after this many failed tasks in a row (default 3)
	LeaseTTL    int           // seconds asked for at claim (0 = the hub's default)
	Beat        time.Duration // heartbeat at least this often (0 = the hub's hint)
	Log         func(string)
}

type lease struct {
	Task struct {
		ID          int64    `json:"id"`
		Project     string   `json:"project"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Acceptance  []string `json:"acceptance"`
		State       string   `json:"state"`
	} `json:"task"`
	Fence          int64 `json:"fence"`
	HeartbeatEvery int   `json:"heartbeat_every_seconds"`
	Claimed        bool  `json:"claimed"`
}

// Outcome is what one task run came to.
type Outcome struct {
	TaskID  int64
	Fence   int64
	RunID   int64
	Branch  string
	Head    string
	State   string // completed | handed_off | lost | no_task
	Reason  string
	Turns   int
	CostUSD float64
}

func (c *Config) logf(f string, a ...any) {
	if c.Log != nil {
		c.Log(fmt.Sprintf(f, a...))
	}
}

func (c *Config) defaults() error {
	if c.Hub == nil || c.Repo == "" || c.WorkRoot == "" || len(c.Agent) == 0 {
		return errors.New("seat: hub, repo, work root and agent are required")
	}
	repo, _ := filepath.Abs(c.Repo)
	root, _ := filepath.Abs(c.WorkRoot)
	if rel, err := filepath.Rel(repo, root); err == nil && !strings.HasPrefix(rel, "..") {
		return errors.New("seat: the work root must be outside the repo")
	}
	c.Repo, c.WorkRoot = repo, root
	if c.Base == "" {
		c.Base = "HEAD"
	}
	if c.LogDir == "" {
		c.LogDir = filepath.Join(c.WorkRoot, "logs")
	}
	if c.Sandbox == "" {
		c.Sandbox = "workspace-write"
	}
	if c.Sandbox != "workspace-write" && c.Sandbox != "unsandboxed" {
		return fmt.Errorf("seat: sandbox must be workspace-write or unsandboxed")
	}
	if c.Sandbox == "workspace-write" && sandbox.Kind() == "none" {
		return errors.New("seat: no working OS sandbox here; pass sandbox unsandboxed explicitly to run without one")
	}
	if c.Timeout <= 0 {
		c.Timeout = 60 * time.Minute
	}
	if c.CIPoll <= 0 {
		c.CIPoll = 15 * time.Second
	}
	if c.Provider == "" {
		c.Provider = "anthropic-api"
	}
	if c.BillingKind == "" {
		c.BillingKind = "api_key"
	}
	if c.MaxFailures <= 0 {
		c.MaxFailures = 3
	}
	if c.Name == "" {
		c.Name = "hub-seat"
	}
	for _, d := range []string{c.WorkRoot, c.LogDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// Run claims and works tasks until ctx ends or MaxFailures tasks in a row fail. With no ready task it waits idle.
func Run(ctx context.Context, c Config, idle time.Duration) error {
	if err := c.defaults(); err != nil {
		return err
	}
	fails := 0
	for ctx.Err() == nil {
		o, err := runOnce(ctx, &c)
		switch {
		case err != nil:
			c.logf("seat: %v", err)
			fails++
		case o.State == "no_task":
			select {
			case <-ctx.Done():
			case <-time.After(idle):
			}
			continue
		case o.State == "completed":
			fails = 0
		default:
			fails++
		}
		if fails >= c.MaxFailures {
			return fmt.Errorf("seat: %d tasks in a row did not complete; stopping", fails)
		}
	}
	return nil
}

// RunOnce claims at most one task and works it to an outcome.
func RunOnce(ctx context.Context, c Config) (Outcome, error) {
	if err := c.defaults(); err != nil {
		return Outcome{}, err
	}
	return runOnce(ctx, &c)
}

func runOnce(ctx context.Context, c *Config) (Outcome, error) {
	var l lease
	filter := map[string]any{"project": c.Project, "labels": c.Labels, "ttl_seconds": c.LeaseTTL}
	if err := c.Hub.do(ctx, "POST", "/claim-next", filter, &l); err != nil {
		return Outcome{}, err
	}
	if !l.Claimed {
		return Outcome{State: "no_task"}, nil
	}
	t := l.Task
	o := Outcome{TaskID: t.ID, Fence: l.Fence, Branch: branchName(t.ID, t.Title)}
	tp := fmt.Sprintf("/tasks/%d", t.ID)
	c.logf("seat: claimed #%d %q (fence %d)", t.ID, t.Title, l.Fence)

	// The lease is kept alive until we finish; losing it cancels the agent.
	work, cancel := context.WithCancel(ctx)
	defer cancel()
	var mu sync.Mutex
	progress := map[string]any{"seat": c.Name, "phase": "setup"}
	lost := make(chan struct{})
	setPhase := func(k string, v any) { mu.Lock(); progress[k] = v; mu.Unlock() }
	go func() {
		every := time.Duration(max(l.HeartbeatEvery, 1)) * time.Second
		if c.Beat > 0 && c.Beat < every {
			every = c.Beat
		}
		tk := time.NewTicker(every)
		defer tk.Stop()
		for {
			select {
			case <-work.Done():
				return
			case <-tk.C:
				mu.Lock()
				p := map[string]any{}
				for k, v := range progress {
					p[k] = v
				}
				mu.Unlock()
				err := c.Hub.do(work, "POST", tp+"/heartbeat", map[string]any{"fence": l.Fence, "progress": p}, nil)
				if IsCode(err, "stale_fence") || IsCode(err, "not_holder") || IsCode(err, "lease_expired") {
					c.logf("seat: lost the lease on #%d: %v", t.ID, err)
					close(lost)
					cancel()
					return
				}
			}
		}
	}()
	isLost := func() bool {
		select {
		case <-lost:
			return true
		default:
			return false
		}
	}
	giveBack := func(reason string) (Outcome, error) {
		o.State, o.Reason = "handed_off", reason
		if isLost() {
			o.State = "lost"
			return o, nil
		}
		note := "Seat " + c.Name + " stopped: " + reason
		if o.Head != "" {
			note += "\nWork so far is on branch " + o.Branch + " at " + o.Head + "."
		}
		err := c.Hub.do(ctx, "POST", tp+"/handoff", map[string]any{"fence": l.Fence, "note": note, "to": c.EscalateTo}, nil)
		return o, err
	}

	dir := filepath.Join(c.WorkRoot, fmt.Sprintf("%d-%d", t.ID, l.Fence))
	if err := addWorktree(work, c.Repo, dir, o.Branch, c.Base); err != nil {
		return giveBack("could not create the worktree: " + err.Error())
	}
	defer removeWorktree(c.Repo, dir)
	base, err := git(work, dir, "rev-parse", "HEAD")
	if err != nil {
		return giveBack(err.Error())
	}
	c.link(work, t.ID, l.Fence, "branch", o.Branch, "", "")

	// The run record and its transcript live on the hub; the local file is a copy.
	var st Stream
	up := c.startRun(ctx, t.ID, l.Fence, o.Branch)
	if up != nil {
		o.RunID = up.runID
	}
	defer func() { up.finish(ctx, &o, &st) }()

	// Run the agent.
	setPhase("phase", "agent")
	st, runErr := c.runAgent(work, dir, prompt(t.ID, t.Title, t.Description, t.Acceptance, o.Branch),
		filepath.Join(c.LogDir, fmt.Sprintf("%d-%d.jsonl", t.ID, l.Fence)), up, func(turns int) { setPhase("turns", turns) })
	o.Turns, o.CostUSD = st.Turns, st.CostUSD
	c.recordUsage(ctx, t.ID, l.Fence, &st)
	if isLost() {
		return giveBack("lease lost")
	}

	// Keep what it produced, even when it failed: the next attempt continues the branch.
	summary := firstLine(st.Result, 2000)
	if _, err := commitAll(work, dir, c.Name, fmt.Sprintf("hub #%d: %s\n\n%s", t.ID, t.Title, summary)); err != nil {
		return giveBack("could not commit: " + err.Error())
	}
	o.Head, _ = git(work, dir, "rev-parse", "HEAD")
	if o.Head != base {
		c.link(work, t.ID, l.Fence, "branch", o.Branch, o.Head, "")
		c.link(work, t.ID, l.Fence, "commit", o.Head, o.Head, "")
	}
	switch {
	case runErr != nil:
		return giveBack("the agent did not finish: " + runErr.Error())
	case !st.Done:
		return giveBack("the agent exited without a result")
	case st.IsError:
		return giveBack(fmt.Sprintf("the agent reported an error (%s): %s", st.Subtype, firstLine(st.Result, 500)))
	case o.Head == base:
		return giveBack("the agent made no changes: " + firstLine(st.Result, 500))
	}

	setPhase("phase", "checks")
	for _, ch := range c.Checks {
		c.runCheck(work, t.ID, l.Fence, dir, o.Head, ch)
	}
	if isLost() {
		return giveBack("lease lost")
	}
	if c.Push != "" {
		setPhase("phase", "push")
		if _, err := git(work, c.Repo, "push", "-q", c.Push, "+refs/heads/"+o.Branch+":refs/heads/"+o.Branch); err != nil {
			return giveBack("push failed: " + err.Error())
		}
		if c.PR != nil && c.RepoName != "" {
			num, url, err := c.PR.Open(work, c.RepoName, o.Branch, fmt.Sprintf("hub #%d: %s", t.ID, t.Title),
				fmt.Sprintf("Task #%d, worked by seat %s.\n\n%s", t.ID, c.Name, summary))
			if err != nil {
				c.logf("seat: PR for #%d: %v", t.ID, err)
			} else {
				c.link(work, t.ID, l.Fence, "pr", fmt.Sprint(num), o.Head, url)
			}
		}
	}

	setPhase("phase", "complete")
	deadline := time.Now().Add(c.CIWait)
	for {
		err := c.Hub.do(work, "POST", tp+"/complete", map[string]any{"fence": l.Fence, "summary": summary}, nil)
		if err == nil {
			o.State = "completed"
			c.logf("seat: completed #%d on %s", t.ID, o.Branch)
			return o, nil
		}
		if !IsCode(err, "evidence_missing") {
			return giveBack("complete refused: " + err.Error())
		}
		if !time.Now().Before(deadline) {
			cur := err.(*APIError).Current
			return giveBack(fmt.Sprintf("required evidence is not passing (missing %v, failing %v)", cur["missing"], cur["failing"]))
		}
		setPhase("phase", "waiting_for_evidence")
		select {
		case <-work.Done():
			return giveBack("stopped while waiting for evidence")
		case <-time.After(c.CIPoll):
		}
	}
}

func prompt(id int64, title, desc string, acceptance []string, branch string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are working task #%d on a fresh git worktree (branch %s).\n\n# %s\n\n", id, branch, title)
	if strings.TrimSpace(desc) != "" {
		b.WriteString(desc + "\n\n")
	}
	if len(acceptance) > 0 {
		b.WriteString("Acceptance criteria:\n")
		for _, a := range acceptance {
			b.WriteString("- " + a + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("Make the change in this directory and run the relevant tests. Do not commit, push or open a pull request: " +
		"the runtime commits your changes, runs the checks and reports to the board. End with a short summary of what you changed " +
		"and anything left undone.\n")
	return b.String()
}

func firstLine(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		s = s[:n] + "…"
	}
	return s
}

// runAgent runs the agent confined to the worktree, writes its stdout to the transcript and parses it.
func (c *Config) runAgent(ctx context.Context, dir, prompt, transcript string, up *uploader, onTurn func(int)) (Stream, error) {
	var st Stream
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	cmd, stop, err := c.command(ctx, dir, c.Agent, c.AllowHosts)
	if err != nil {
		return st, err
	}
	defer stop()
	f, err := os.Create(transcript)
	if err != nil {
		return st, err
	}
	defer f.Close()
	cmd.Stdin = strings.NewReader(prompt)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return st, err
	}
	var stderr tail
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return st, err
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			sandbox.Kill(cmd)
		case <-done:
		}
	}()
	sc := bufio.NewScanner(io.TeeReader(out, f))
	sc.Buffer(make([]byte, 64<<10), 16<<20)
	turns := 0
	for sc.Scan() {
		up.line(sc.Bytes())
		st.Add(sc.Bytes())
		if st.Turns != turns {
			turns = st.Turns
			onTurn(turns)
		}
	}
	err = cmd.Wait()
	close(done)
	if ctx.Err() != nil {
		return st, fmt.Errorf("stopped: %v", ctx.Err())
	}
	if err != nil {
		return st, fmt.Errorf("%v: %s", err, stderr.String())
	}
	return st, nil
}

func (c *Config) recordUsage(ctx context.Context, id, fence int64, st *Stream) {
	for i, u := range st.FinalUsage() {
		in := map[string]any{"task_id": id, "fence": fence, "provider_id": c.Provider, "model": u.Model, "billing_kind": c.BillingKind,
			"source": "stream", "input_tokens": u.Input, "cache_read_tokens": u.CacheRead, "cache_write_tokens": u.CacheWrite,
			"output_tokens": u.Output}
		if u.CostMicros != nil {
			in["cost_micros"], in["cost_basis"] = *u.CostMicros, "client_estimate"
		}
		if st.SessionID != "" {
			in["external_ref"] = fmt.Sprintf("%s/%s/%d", st.SessionID, u.Model, i)
		}
		if err := c.Hub.do(ctx, "POST", "/usage", in, nil); err != nil {
			c.logf("seat: usage for #%d: %v", id, err)
		}
	}
}

func (c *Config) link(ctx context.Context, id, fence int64, kind, ref, head, url string) {
	if c.RepoName == "" {
		return
	}
	in := map[string]any{"kind": kind, "repo": c.RepoName, "ref": ref, "fence": fence}
	if head != "" {
		in["head_sha"] = head
	}
	if url != "" {
		in["url"] = url
	}
	if err := c.Hub.do(ctx, "POST", fmt.Sprintf("/tasks/%d/links", id), in, nil); err != nil {
		c.logf("seat: link %s for #%d: %v", kind, id, err)
	}
}

// runCheck runs one check confined to the worktree with no network and records pass or fail as evidence for head.
func (c *Config) runCheck(ctx context.Context, id, fence int64, dir, head string, ch Check) {
	status, summary := "pass", ""
	cmd, stop, err := c.command(ctx, dir, ch.Argv, nil)
	if err == nil {
		var out tail
		cmd.Stdout, cmd.Stderr = &out, &out
		err = cmd.Run()
		summary = out.String()
		stop()
	}
	if err != nil {
		status, summary = "fail", firstLine(err.Error()+"\n"+summary, 1500)
	} else {
		summary = firstLine(summary, 1500)
	}
	in := map[string]any{"kind": ch.Kind, "status": status, "summary": summary, "fence": fence,
		"data": map[string]any{"head_sha": head, "argv": ch.Argv, "seat": c.Name}}
	if c.RepoName == "" {
		delete(in["data"].(map[string]any), "head_sha") // no branch link to match it against
	}
	if err := c.Hub.do(ctx, "POST", fmt.Sprintf("/tasks/%d/evidence", id), in, nil); err != nil {
		c.logf("seat: evidence %s for #%d: %v", ch.Kind, id, err)
	}
}

// command builds argv confined to dir (read-write) with the repo's .git readable, and an egress proxy for hosts.
func (c *Config) command(ctx context.Context, dir string, argv []string, hosts []string) (cmdT, func(), error) {
	env := map[string]string{"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1", "GIT_CONFIG_NOSYSTEM": "1"}
	for k, v := range c.Env {
		env[k] = v
	}
	spec := sandbox.Spec{Level: c.Sandbox, Paths: []string{dir}, ReadPaths: []string{filepath.Join(c.Repo, ".git")}, Cwd: dir,
		Argv: argv, Env: env, Self: c.Self}
	stop := func() {}
	if c.Sandbox != "unsandboxed" {
		env["IS_SANDBOX"] = "1"
		spec.ProxySock, spec.ProxyTCP, stop = netproxy.Start(ctx, hosts, func(s string) { c.logf("seat: %s", s) })
	}
	cmd, err := sandbox.Command(spec)
	if err != nil {
		stop()
		return nil, func() {}, err
	}
	return cmd, stop, nil
}

// tail keeps the last 8 KiB written to it.
type tail struct {
	mu  sync.Mutex
	buf []byte
}

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > 8<<10 {
		t.buf = t.buf[len(t.buf)-8<<10:]
	}
	return len(p), nil
}

func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.TrimSpace(string(t.buf))
}
