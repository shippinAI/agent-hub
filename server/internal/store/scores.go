package store

import (
	"context"
	"slices"
	"time"
)

// Run outcomes, computed from the record (nothing is a stored verdict):
//   - accepted: the run completed and a later completion of the task by someone else moved it on (the next lane,
//     such as review, finished it), and no later run in the same lane reworked it
//   - reworked: the run completed but a later run in the same lane on the same task started (it came back)
//   - pending: completed, not yet accepted or reworked
//   - handed_off | failed | lost: the run did not complete
//
// Success rate = accepted / (accepted + reworked + handed_off + failed + lost); pending runs are not scored yet.

// ScoreFilter narrows Scores.
type ScoreFilter struct {
	GroupBy   string    `json:"group_by"` // principal (default) | adapter | model | lane | project
	Principal string    `json:"principal,omitempty"`
	Project   string    `json:"project,omitempty"`
	Lane      string    `json:"lane,omitempty"`
	Labels    []string  `json:"labels,omitempty"`
	Since     time.Time `json:"since,omitempty"`
}

// Score is one group's outcome counts, success rate and cost.
type Score struct {
	Key             string   `json:"key"`
	Runs            int64    `json:"runs"`
	Accepted        int64    `json:"accepted"`
	Reworked        int64    `json:"reworked"`
	Pending         int64    `json:"pending"`
	HandedOff       int64    `json:"handed_off"`
	Failed          int64    `json:"failed"`
	Lost            int64    `json:"lost"`
	Scored          int64    `json:"scored"`
	SuccessPct      *float64 `json:"success_pct"` // nil until a run is scored
	CostMicros      int64    `json:"cost_micros"`
	CostPerAccepted *int64   `json:"cost_per_accepted_micros"`
	Turns           int64    `json:"turns"`
}

var scoreGroups = map[string]string{"principal": "r.principal_id", "adapter": "r.adapter", "model": "r.model", "lane": "r.lane", "project": "t.project"}

// scoreSQL classifies every finished run; $1 project, $2 labels, $3 since, $4 principal, $5 lane (” = any).
const scoreSQL = `WITH o AS (
	SELECT r.*, t.project, CASE
		WHEN r.status <> 'completed' THEN r.status
		WHEN EXISTS (SELECT 1 FROM runs r2 WHERE r2.task_id=r.task_id AND r2.lane=r.lane AND r2.lane<>'' AND r2.started_at > r.ended_at) THEN 'reworked'
		WHEN EXISTS (SELECT 1 FROM events e WHERE e.task_id=r.task_id AND e.action='task.completed' AND e.at > r.ended_at AND e.actor<>r.principal_id) THEN 'accepted'
		ELSE 'pending' END AS outcome,
		(SELECT coalesce(sum(u.cost_micros),0) FROM usage_events u WHERE u.task_id=r.task_id AND u.fence=r.fence AND u.principal_id=r.principal_id) AS cost
	FROM runs r JOIN tasks t ON t.id=r.task_id
	WHERE r.status <> 'running' AND ($1='' OR t.project=$1) AND t.labels @> $2 AND r.started_at >= $3
		AND ($4='' OR r.principal_id=$4) AND ($5='' OR r.lane=$5))`

func scoreSelect(key string) string {
	return scoreSQL + ` SELECT ` + key + `, count(*),
		count(*) FILTER (WHERE outcome='accepted'), count(*) FILTER (WHERE outcome='reworked'), count(*) FILTER (WHERE outcome='pending'),
		count(*) FILTER (WHERE outcome='handed_off'), count(*) FILTER (WHERE outcome='failed'), count(*) FILTER (WHERE outcome='lost'),
		coalesce(sum(cost),0), coalesce(sum(turns),0) FROM o r GROUP BY 1 ORDER BY 1`
}

func scanScores(ctx context.Context, q querier, sql string, args ...any) ([]Score, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Score{}
	for rows.Next() {
		var s Score
		if err := rows.Scan(&s.Key, &s.Runs, &s.Accepted, &s.Reworked, &s.Pending, &s.HandedOff, &s.Failed, &s.Lost, &s.CostMicros, &s.Turns); err != nil {
			return nil, err
		}
		s.Scored = s.Runs - s.Pending
		if s.Scored > 0 {
			pct := float64(s.Accepted) * 100 / float64(s.Scored)
			s.SuccessPct = &pct
		}
		if s.Accepted > 0 {
			c := s.CostMicros / s.Accepted
			s.CostPerAccepted = &c
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Scores groups finished runs by principal, adapter, model, lane or project.
func (s *Store) Scores(ctx context.Context, f ScoreFilter) ([]Score, error) {
	if f.GroupBy == "" {
		f.GroupBy = "principal"
	}
	key, ok := scoreGroups[f.GroupBy]
	if !ok {
		return nil, errInvalid("group_by must be principal, adapter, model, lane or project")
	}
	if f.GroupBy == "project" {
		key = "r.project" // the CTE carries the task's project as a run column
	}
	return scanScores(ctx, s.DB, scoreSelect(key), f.Project, cleanLabels(f.Labels), f.Since, f.Principal, f.Lane)
}

// routedAway reports whether claim_next should skip this task for the caller: routing is on (route_min_runs > 0),
// the caller has at least that many scored runs in the task's project and the lane it would claim into, and its
// success rate there is below route_min_success_pct. Tasks assigned to the caller are never routed away.
func (w *W) routedAway(st Settings, t Task, activeState string, cache map[[2]string]*Score) (bool, *Score, error) {
	minRuns := st.Int("route_min_runs", 0)
	if minRuns <= 0 || (t.Assignee != nil && *t.Assignee == w.P.ID) {
		return false, nil, nil
	}
	k := [2]string{t.Project, activeState}
	sc, ok := cache[k]
	if !ok {
		got, err := scanScores(w.ctx, w.tx, scoreSelect("r.principal_id"), t.Project, []string{}, time.Time{}, w.P.ID, activeState)
		if err != nil {
			return false, nil, err
		}
		i := slices.IndexFunc(got, func(s Score) bool { return s.Key == w.P.ID })
		if i >= 0 {
			sc = &got[i]
		}
		cache[k] = sc
	}
	if sc == nil || sc.Scored < int64(minRuns) || sc.SuccessPct == nil {
		return false, sc, nil
	}
	return *sc.SuccessPct < float64(st.Int("route_min_success_pct", 50)), sc, nil
}
