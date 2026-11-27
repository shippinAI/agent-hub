package store

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// taskRef finds a task named in a branch ("hub-12", "hub/12-fix") or in PR text ("hub#12").
var (
	branchTaskRef = regexp.MustCompile(`(?i)(?:^|[/_-])hub[-/_]?(\d{1,12})(?:$|[/_-])`)
	textTaskRef   = regexp.MustCompile(`(?i)\bhub#(\d{1,12})\b`)
)

// CheckEvidenceKind is the evidence kind a completed GitHub check run attaches: "ci." plus the check name,
// lowercased, with anything outside [a-z0-9_.-] turned into "-" (check "go" -> ci.go, "Lint / web" -> ci.lint-web).
func CheckEvidenceKind(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '.' {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	s := strings.Trim(b.String(), "-.")
	if len(s) > 38 {
		s = strings.Trim(s[:38], "-.")
	}
	if s == "" {
		s = "check"
	}
	return "ci." + s
}

// checkStatus maps a check conclusion to evidence; neutral and skipped prove nothing.
func checkStatus(conclusion string) (string, bool) {
	switch conclusion {
	case "success":
		return "pass", true
	case "failure", "timed_out", "cancelled", "action_required", "startup_failure", "stale":
		return "fail", true
	}
	return "", false
}

type ghRepo struct {
	FullName string `json:"full_name"`
}

type ghPayload struct {
	Action     string `json:"action"`
	Repository ghRepo `json:"repository"`
	// push
	Ref     string `json:"ref"`
	After   string `json:"after"`
	Deleted bool   `json:"deleted"`
	Commits []struct {
		ID      string `json:"id"`
		URL     string `json:"url"`
		Message string `json:"message"`
	} `json:"commits"`
	// pull_request
	PullRequest *struct {
		Number  int64  `json:"number"`
		HTMLURL string `json:"html_url"`
		Title   string `json:"title"`
		Body    string `json:"body"`
		State   string `json:"state"`
		Draft   bool   `json:"draft"`
		Merged  bool   `json:"merged"`
		Head    struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
	} `json:"pull_request"`
	// check_run
	CheckRun *struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		HeadSHA    string `json:"head_sha"`
		HTMLURL    string `json:"html_url"`
		DetailsURL string `json:"details_url"`
		CheckSuite struct {
			HeadBranch string `json:"head_branch"`
		} `json:"check_suite"`
		PullRequests []struct {
			Number int64 `json:"number"`
		} `json:"pull_requests"`
	} `json:"check_run"`
}

// IngestGitHub applies one GitHub webhook delivery (already signature-checked) as the github principal.
// A delivery id seen before is acknowledged and not applied again.
func (s *Store) IngestGitHub(ctx context.Context, delivery, event string, body []byte) (map[string]any, error) {
	var p ghPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, errInvalid("payload is not JSON: %v", err)
	}
	out, err := s.write(ctx, &Principal{ID: "github", DisplayName: "GitHub", Kind: "service", Enabled: true}, "", "github_webhook", nil,
		func(w *W) (any, error) {
			var enabled bool
			if err := w.row(`SELECT enabled FROM principals WHERE id='github'`).Scan(&enabled); err != nil || !enabled {
				return nil, fail(403, "policy_denied", "the github principal is missing or disabled")
			}
			if _, err := w.exec(`DELETE FROM webhook_deliveries WHERE received_at < clock_timestamp() - interval '7 days'`); err != nil {
				return nil, err
			}
			tag, err := w.exec(`INSERT INTO webhook_deliveries(id,source,event) VALUES($1,'github',$2) ON CONFLICT DO NOTHING`, delivery, event)
			if err != nil {
				return nil, err
			}
			if tag.RowsAffected() == 0 {
				return map[string]any{"duplicate": true}, nil
			}
			g := ghIngest{w: w, repo: p.Repository.FullName, touched: map[int64]bool{}}
			if g.repo == "" && event != "ping" {
				return map[string]any{"ignored": "no repository"}, nil
			}
			switch event {
			case "ping":
			case "push":
				err = g.push(p)
			case "pull_request":
				err = g.pullRequest(p)
			case "check_run":
				err = g.checkRun(p)
			default:
				return map[string]any{"ignored": event}, nil
			}
			if err != nil {
				return nil, err
			}
			ids := []int64{}
			for id := range g.touched {
				ids = append(ids, id)
			}
			slices.Sort(ids)
			return map[string]any{"event": event, "tasks": ids, "evidence": g.evidence}, nil
		})
	if err != nil {
		return nil, err
	}
	return out.(map[string]any), nil
}

type ghIngest struct {
	w        *W
	repo     string
	touched  map[int64]bool
	evidence int
}

// tasksBy returns ids of tasks with a link of kind/ref in this repo.
func (g *ghIngest) tasksBy(kind, ref string) ([]int64, error) {
	rows, err := g.w.tx.Query(g.w.ctx, `SELECT DISTINCT task_id FROM task_links WHERE repo=$1 AND kind=$2 AND ref=$3 ORDER BY task_id`,
		g.repo, kind, ref)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// named returns task ids a branch name or text points at, if those tasks exist.
func (g *ghIngest) named(branch, text string) ([]int64, error) {
	var ids []int64
	add := func(m []string) {
		if id, err := strconv.ParseInt(m[1], 10, 64); err == nil && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	if m := branchTaskRef.FindStringSubmatch(branch); m != nil {
		add(m)
	}
	for _, m := range textTaskRef.FindAllStringSubmatch(text, 10) {
		add(m)
	}
	var out []int64
	for _, id := range ids {
		var ok bool
		if err := g.w.row(`SELECT EXISTS(SELECT 1 FROM tasks WHERE id=$1)`, id).Scan(&ok); err != nil {
			return nil, err
		}
		if ok {
			out = append(out, id)
		}
	}
	return out, nil
}

func (g *ghIngest) task(id int64) (Task, error) {
	g.touched[id] = true
	return getTask(g.w.ctx, g.w.tx, id, true)
}

func union(a ...[]int64) []int64 {
	var out []int64
	for _, l := range a {
		for _, id := range l {
			if !slices.Contains(out, id) {
				out = append(out, id)
			}
		}
	}
	slices.Sort(out)
	return out
}

func (g *ghIngest) push(p ghPayload) error {
	branch, ok := strings.CutPrefix(p.Ref, "refs/heads/")
	if !ok || p.Deleted || !gitSHA.MatchString(p.After) {
		return nil
	}
	linked, err := g.tasksBy("branch", branch)
	if err != nil {
		return err
	}
	named, err := g.named(branch, "")
	if err != nil {
		return err
	}
	for _, id := range union(linked, named) {
		t, err := g.task(id)
		if err != nil {
			return err
		}
		if _, err := g.w.link(t, TaskLink{Kind: "branch", Repo: g.repo, Ref: branch, HeadSHA: p.After,
			URL: "https://github.com/" + g.repo + "/tree/" + branch}); err != nil {
			return err
		}
		for i, c := range p.Commits {
			if i == 20 || !gitSHA.MatchString(c.ID) {
				break
			}
			subject, _, _ := strings.Cut(c.Message, "\n")
			if len(subject) > 200 {
				subject = subject[:200]
			}
			data, _ := json.Marshal(map[string]any{"subject": subject, "branch": branch})
			if _, err := g.w.link(t, TaskLink{Kind: "commit", Repo: g.repo, Ref: c.ID, URL: httpsOnly(c.URL), Data: data}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *ghIngest) pullRequest(p ghPayload) error {
	pr := p.PullRequest
	if pr == nil {
		return nil
	}
	num := strconv.FormatInt(pr.Number, 10)
	byPR, err := g.tasksBy("pr", num)
	if err != nil {
		return err
	}
	byBranch, err := g.tasksBy("branch", pr.Head.Ref)
	if err != nil {
		return err
	}
	named, err := g.named(pr.Head.Ref, pr.Title+"\n"+pr.Body)
	if err != nil {
		return err
	}
	state := pr.State
	switch {
	case pr.Merged:
		state = "merged"
	case pr.State == "open" && pr.Draft:
		state = "draft"
	}
	head := strings.ToLower(pr.Head.SHA)
	if !gitSHA.MatchString(head) {
		head = ""
	}
	title := pr.Title
	if len(title) > 200 {
		title = title[:200]
	}
	data, _ := json.Marshal(map[string]any{"title": title, "branch": pr.Head.Ref})
	for _, id := range union(byPR, byBranch, named) {
		t, err := g.task(id)
		if err != nil {
			return err
		}
		if _, err := g.w.link(t, TaskLink{Kind: "pr", Repo: g.repo, Ref: num, URL: httpsOnly(pr.HTMLURL), State: state,
			HeadSHA: head, Data: data}); err != nil {
			return err
		}
		if pr.Head.Ref != "" {
			if _, err := g.w.link(t, TaskLink{Kind: "branch", Repo: g.repo, Ref: pr.Head.Ref, HeadSHA: head,
				URL: "https://github.com/" + g.repo + "/tree/" + pr.Head.Ref}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *ghIngest) checkRun(p ghPayload) error {
	cr := p.CheckRun
	if cr == nil || p.Action != "completed" || cr.Name == "" {
		return nil
	}
	head := strings.ToLower(cr.HeadSHA)
	if !gitSHA.MatchString(head) {
		return nil
	}
	var lists [][]int64
	if cr.CheckSuite.HeadBranch != "" {
		l, err := g.tasksBy("branch", cr.CheckSuite.HeadBranch)
		if err != nil {
			return err
		}
		lists = append(lists, l)
	}
	for _, pr := range cr.PullRequests {
		l, err := g.tasksBy("pr", strconv.FormatInt(pr.Number, 10))
		if err != nil {
			return err
		}
		lists = append(lists, l)
	}
	l, err := g.tasksBy("commit", head)
	if err != nil {
		return err
	}
	lists = append(lists, l)
	url := httpsOnly(cr.HTMLURL)
	if url == "" {
		url = httpsOnly(cr.DetailsURL)
	}
	name := cr.Name
	if len(name) > 255 {
		name = name[:255]
	}
	for _, id := range union(lists...) {
		t, err := g.task(id)
		if err != nil {
			return err
		}
		data, _ := json.Marshal(map[string]any{"conclusion": cr.Conclusion})
		if _, err := g.w.link(t, TaskLink{Kind: "check_run", Repo: g.repo, Ref: name, URL: url, State: cr.Conclusion,
			HeadSHA: head, Data: data}); err != nil {
			return err
		}
		status, ok := checkStatus(cr.Conclusion)
		if !ok {
			continue
		}
		// Evidence flows only through a branch or PR that a hub principal linked (link_git), never through a link
		// the webhook made from a name like hub-12: anyone who can push could name a branch. And only the newest
		// commit of such a branch proves anything; a check on an older commit is history.
		var explicit, stale bool
		if err := g.w.row(`SELECT EXISTS(SELECT 1 FROM task_links WHERE task_id=$1 AND repo=$2 AND author<>'github'
				AND ((kind='branch' AND ref=$3) OR (kind='pr' AND ref = ANY($4)))),
			EXISTS(SELECT 1 FROM task_links WHERE task_id=$1 AND kind='branch' AND author<>'github' AND head_sha<>'')
				AND NOT EXISTS(SELECT 1 FROM task_links WHERE task_id=$1 AND kind='branch' AND author<>'github' AND head_sha=$5)`,
			t.ID, g.repo, cr.CheckSuite.HeadBranch, prNums(cr.PullRequests), head).Scan(&explicit, &stale); err != nil {
			return err
		}
		if !explicit || stale {
			continue
		}
		ev, _ := json.Marshal(map[string]any{"head_sha": head, "check": cr.Name, "conclusion": cr.Conclusion, "repo": g.repo})
		if _, err := g.w.addEvidence(t, EvidenceInput{Kind: CheckEvidenceKind(cr.Name), Status: status, Ref: url,
			Summary: fmt.Sprintf("GitHub check %q: %s on %s", name, cr.Conclusion, head[:12]), Data: ev}); err != nil {
			return err
		}
		g.evidence++
	}
	return nil
}

func prNums(prs []struct {
	Number int64 `json:"number"`
}) []string {
	out := []string{}
	for _, p := range prs {
		out = append(out, strconv.FormatInt(p.Number, 10))
	}
	return out
}

func httpsOnly(u string) string {
	if strings.HasPrefix(u, "https://") && len(u) <= 2000 {
		return u
	}
	return ""
}
