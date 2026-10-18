package subs

import (
	"context"
	"fmt"
)

// Route is which subscription and worker would take a role's task now (never a secret).
type Route struct {
	Role         string   `json:"role"`
	ChatApp      string   `json:"chat_app"`
	Subscription string   `json:"subscription_id"`
	Label        string   `json:"label"`
	Kind         string   `json:"kind"`
	Worker       *string  `json:"worker_principal_id"`
	Skipped      []string `json:"skipped"`
	Note         string   `json:"note,omitempty"`
}

// Resolve walks a role's bindings in priority order (fallbacks after their primary), skipping disabled,
// over-budget and offline subscriptions. A cli_login worker counts as online when its machine's last
// completed call is within online_window_seconds or the worker principal was seen in that window.
func (s *Service) Resolve(ctx context.Context, role, chatApp string) (Route, error) {
	set, _ := s.St.LoadSettings(ctx)
	win := set.Int("online_window_seconds", 120)
	rows, err := s.St.DB.Query(ctx, `WITH b AS (
		SELECT rb.chat_app, rb.priority, 0 AS fb, rb.subscription_id AS sid FROM role_bindings rb WHERE rb.role_id=$1 AND rb.enabled AND ($2='' OR rb.chat_app=$2)
		UNION ALL SELECT rb.chat_app, rb.priority, 1, rb.fallback_subscription_id FROM role_bindings rb
		WHERE rb.role_id=$1 AND rb.enabled AND rb.fallback_subscription_id IS NOT NULL AND ($2='' OR rb.chat_app=$2))
		SELECT b.chat_app, s.id::text, s.label, s.kind, s.worker_principal_id, s.enabled,
		coalesce((s.spend->>'usd')::numeric >= (s.budget->>'usd')::numeric, false),
		CASE WHEN s.kind='api_key' THEN s.secret_id IS NOT NULL ELSE
		  coalesce((SELECT m.last_exec_ok_at > now()-make_interval(secs => $3) FROM machines m WHERE m.id=s.machine_id), false)
		  OR coalesce((SELECT p.last_seen_at > now()-make_interval(secs => $3) FROM principals p WHERE p.id=s.worker_principal_id), false) END
		FROM b JOIN subscriptions s ON s.id=b.sid ORDER BY b.priority, b.fb`, role, chatApp, win)
	if err != nil {
		return Route{}, err
	}
	defer rows.Close()
	r := Route{Role: role, Skipped: []string{}}
	for rows.Next() {
		var app, id, label, kind string
		var worker *string
		var enabled, over, online bool
		if err := rows.Scan(&app, &id, &label, &kind, &worker, &enabled, &over, &online); err != nil {
			return r, err
		}
		why := map[bool]string{true: ""}[enabled && !over && online]
		switch {
		case !enabled:
			why = "disabled"
		case over:
			why = "over budget"
		case !online:
			why = "offline"
		}
		if why != "" {
			r.Skipped = append(r.Skipped, fmt.Sprintf("%s (%s)", label, why))
			continue
		}
		r.ChatApp, r.Subscription, r.Label, r.Kind, r.Worker = app, id, label, kind, worker
		if len(r.Skipped) > 0 {
			r.Note = fmt.Sprintf("fell back to %s because %v", label, r.Skipped)
		}
		return r, nil
	}
	return r, fmt.Errorf("no available subscription for role %s (skipped: %v)", role, r.Skipped)
}
