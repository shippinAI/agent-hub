package store

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// Budget caps cost (micro-USD) or tokens for a scope and period. Over the limit, new claims in that scope are
// refused with budget_exceeded. Only humans edit budgets (an agent cannot raise its own ceiling).
type Budget struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	ScopeKind  string `json:"scope_kind"` // all | principal | project | task
	ScopeID    string `json:"scope_id"`
	Metric     string `json:"metric"` // cost_micros | tokens
	Period     string `json:"period"` // day | week | month | total
	LimitValue int64  `json:"limit_value"`
	Enabled    bool   `json:"enabled"`
	Version    int64  `json:"version"`
}

// BudgetStatus is a budget with what has been spent in its open period.
type BudgetStatus struct {
	Budget
	PeriodStart time.Time `json:"period_start"`
	Spent       int64     `json:"spent"`
	Over        bool      `json:"over"`
}

const budgetCols = `id,label,scope_kind,scope_id,metric,period,limit_value,enabled,version`

func scanBudget(r interface{ Scan(...any) error }) (Budget, error) {
	var b Budget
	err := r.Scan(&b.ID, &b.Label, &b.ScopeKind, &b.ScopeID, &b.Metric, &b.Period, &b.LimitValue, &b.Enabled, &b.Version)
	return b, err
}

var budgetID = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)

// periodStart is the start of the open period, in UTC.
func periodStart(period string, now time.Time) time.Time {
	now = now.UTC()
	d := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	switch period {
	case "day":
		return d
	case "week":
		return d.AddDate(0, 0, -((int(d.Weekday()) + 6) % 7)) // Monday
	case "month":
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	return time.Unix(0, 0).UTC()
}

// budgetStatus evaluates every enabled budget that covers the principal and, when given, the task.
func (w *W) budgetStatus(principal string, t *Task) ([]BudgetStatus, error) {
	project, task := "", ""
	if t != nil {
		project, task = t.Project, strconv.FormatInt(t.ID, 10)
	}
	rows, err := w.tx.Query(w.ctx, `SELECT `+budgetCols+` FROM budgets WHERE enabled AND (scope_kind='all'
		OR (scope_kind='principal' AND scope_id=$1) OR (scope_kind='project' AND scope_id=$2) OR (scope_kind='task' AND scope_id=$3))
		ORDER BY id`, principal, project, task)
	if err != nil {
		return nil, err
	}
	var bs []Budget
	for rows.Next() {
		b, err := scanBudget(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		bs = append(bs, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := []BudgetStatus{}
	for _, b := range bs {
		st, err := spent(w.ctx, w.tx, b, time.Now())
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, nil
}

// spent sums a budget's metric over its open period.
func spent(ctx context.Context, q querier, b Budget, now time.Time) (BudgetStatus, error) {
	st := BudgetStatus{Budget: b, PeriodStart: periodStart(b.Period, now)}
	metric := `coalesce(sum(cost_micros),0)`
	if b.Metric == "tokens" {
		metric = `coalesce(sum(input_tokens + cache_write_tokens + output_tokens),0)`
	}
	err := q.QueryRow(ctx, `SELECT `+metric+` FROM usage_events WHERE occurred_at >= $1 AND (
		$2='all' OR ($2='principal' AND principal_id=$3) OR ($2='project' AND project=$3) OR ($2='task' AND task_id::text=$3))`,
		st.PeriodStart, b.ScopeKind, b.ScopeID).Scan(&st.Spent)
	st.Over = b.Enabled && st.Spent >= b.LimitValue
	return st, err
}

// fireAlerts logs budget.exceeded once per budget and period.
func (w *W) fireAlerts(st []BudgetStatus) error {
	for _, b := range st {
		if !b.Over {
			continue
		}
		tag, err := w.exec(`INSERT INTO budget_alerts(budget_id,period_start,value) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`,
			b.ID, b.PeriodStart, b.Spent)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 1 {
			if err := w.Event("budget.exceeded", nil, nil, map[string]any{"budget_id": b.ID, "scope_kind": b.ScopeKind,
				"scope_id": b.ScopeID, "metric": b.Metric, "limit": b.LimitValue, "spent": b.Spent, "period_start": b.PeriodStart}); err != nil {
				return err
			}
		}
	}
	return nil
}

// budgetGate refuses a claim when any budget covering the caller or the task is used up.
func (w *W) budgetGate(t *Task) error {
	st, err := w.budgetStatus(w.P.ID, t)
	if err != nil {
		return err
	}
	var over []BudgetStatus
	for _, b := range st {
		if b.Over {
			over = append(over, b)
		}
	}
	if len(over) == 0 {
		return nil
	}
	e := fail(409, "budget_exceeded", "budget %q is used up (%d of %d %s this %s); a human must raise or disable it",
		over[0].ID, over[0].Spent, over[0].LimitValue, over[0].Metric, over[0].Period)
	e.Current = map[string]any{"budgets": over}
	return e
}

// PutBudget creates (version 0) or updates a budget. Humans only.
func (s *Store) PutBudget(ctx context.Context, p *Principal, reqID string, b Budget) (any, error) {
	return s.write(ctx, p, reqID, "put_budget", b, func(w *W) (any, error) {
		if err := w.admin("budgets.put"); err != nil {
			return nil, err
		}
		switch {
		case !budgetID.MatchString(b.ID):
			return nil, errInvalid("budget id must be lowercase letters, digits, . _ -")
		case !slices.Contains([]string{"all", "principal", "project", "task"}, b.ScopeKind):
			return nil, errInvalid("scope_kind must be all, principal, project or task")
		case (b.ScopeKind == "all") != (b.ScopeID == ""):
			return nil, errInvalid("scope_id is required for a scoped budget and empty for scope all")
		case !slices.Contains([]string{"cost_micros", "tokens"}, b.Metric):
			return nil, errInvalid("metric must be cost_micros or tokens")
		case !slices.Contains([]string{"day", "week", "month", "total"}, b.Period):
			return nil, errInvalid("period must be day, week, month or total")
		case b.LimitValue <= 0:
			return nil, errInvalid("limit_value must be positive")
		case len(b.Label) > 200:
			return nil, errInvalid("label is limited to 200 chars")
		}
		err := w.versioned("budgets", b.ID, b.Version, func() error {
			_, e := w.exec(`INSERT INTO budgets(id,label,scope_kind,scope_id,metric,period,limit_value,enabled) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
				b.ID, b.Label, b.ScopeKind, b.ScopeID, b.Metric, b.Period, b.LimitValue, b.Enabled)
			return e
		}, func() error {
			_, e := w.exec(`UPDATE budgets SET label=$2,scope_kind=$3,scope_id=$4,metric=$5,period=$6,limit_value=$7,enabled=$8,
				version=version+1 WHERE id=$1`, b.ID, b.Label, b.ScopeKind, b.ScopeID, b.Metric, b.Period, b.LimitValue, b.Enabled)
			return e
		})
		if err != nil {
			return nil, err
		}
		out, err := scanBudget(w.row(`SELECT `+budgetCols+` FROM budgets WHERE id=$1`, b.ID))
		if err != nil {
			return nil, err
		}
		return out, w.Event("admin.budget.put", nil, nil, out)
	})
}

// Budgets lists every budget with its open-period spend.
func (s *Store) Budgets(ctx context.Context) ([]BudgetStatus, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+budgetCols+` FROM budgets ORDER BY id`)
	if err != nil {
		return nil, err
	}
	var bs []Budget
	for rows.Next() {
		b, err := scanBudget(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		bs = append(bs, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := []BudgetStatus{}
	for _, b := range bs {
		st, err := spent(ctx, s.DB, b, time.Now())
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, nil
}
