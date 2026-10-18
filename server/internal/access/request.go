package access

import (
	"context"
	"encoding/json"

	"devagotchi/bridge/scope"
	"agenthub/internal/store"
)

// Req is request_access. A preset expands into its items (one request row each, one group).
type Req struct {
	Machine      string         `json:"machine"`
	Capability   string         `json:"capability"`
	Selector     map[string]any `json:"selector"`
	SandboxLevel string         `json:"sandbox_level"`
	Duration     string         `json:"duration"`
	Reason       string         `json:"reason"`
	TaskID       *int64         `json:"task_id"`
	GuideRunID   string         `json:"guide_run_id"`
	Preset       string         `json:"preset"`
	UsesMax      *int           `json:"uses_max"`
}

type item struct {
	Capability   string         `json:"capability"`
	Selector     map[string]any `json:"selector"`
	SandboxLevel string         `json:"sandbox_level"`
}

// Request files an access request (pending until a human decides).
func (s *Service) Request(ctx context.Context, p *store.Principal, reqID string, r Req) (any, error) {
	return s.St.Write(ctx, p, reqID, "access_request", r, func(w *store.W) (any, error) {
		if err := w.Can("access.request"); err != nil {
			return nil, err
		}
		if len(r.Reason) < 20 {
			return nil, store.Fail(422, "reason_too_short", "reason must be at least 20 characters and say why")
		}
		var display, kind string
		if err := w.Row(`SELECT coalesce(display_server,''),kind FROM machines WHERE id=$1 AND NOT disabled`, r.Machine).Scan(&display, &kind); err != nil {
			return nil, store.ErrNotFound("machine " + r.Machine)
		}
		items, dur, err := expand(w, &r)
		if err != nil {
			return nil, err
		}
		if err := checkDuration(w, r, dur); err != nil {
			return nil, err
		}
		set, _ := w.Settings()
		var group string
		_ = w.Row(`SELECT gen_random_uuid()::text`).Scan(&group)
		ids := []string{}
		for _, it := range items {
			if err := checkItem(w, r.Machine, it, items); err != nil {
				return nil, err
			}
			risks, err := snapshot(w, it, dur, display, kind)
			if err != nil {
				return nil, err
			}
			var collapsed bool
			_ = w.Row(`SELECT EXISTS(SELECT 1 FROM access_requests WHERE requester=$1 AND machine_id=$2 AND capability=$3 AND selector=$4
				AND status='denied' AND decided_at > now() - make_interval(mins => $5))`, w.P.ID, r.Machine, it.Capability, jsonb(it.Selector),
				set.Int("deny_cooldown_minutes", 10)).Scan(&collapsed)
			var id string
			err = w.Row(`INSERT INTO access_requests(group_id,requester,task_id,guide_run_id,machine_id,capability,selector,sandbox_level,duration,
				uses_max,preset_id,reason,risk_snapshot,collapsed,expires_at)
				VALUES($1::uuid,$2,$3,nullif($4,'')::uuid,$5,$6,$7,$8,$9,$10,nullif($11,''),$12,$13,$14,now()+make_interval(mins => $15)) RETURNING id::text`,
				group, w.P.ID, r.TaskID, r.GuideRunID, r.Machine, it.Capability, jsonb(it.Selector), it.SandboxLevel, jsonb(dur),
				r.UsesMax, r.Preset, r.Reason, risks, collapsed, set.Int("request_expiry_minutes", 30)).Scan(&id)
			if err != nil {
				return nil, store.ErrInvalid("%v", err)
			}
			ids = append(ids, id)
		}
		return map[string]any{"group_id": group, "request_ids": ids, "status": "pending"},
			w.Event("access.requested", r.TaskID, nil, map[string]any{"group": group, "machine": r.Machine, "requester": w.P.ID, "items": len(ids), "preset": r.Preset})
	})
}

func expand(w *store.W, r *Req) ([]item, string, error) {
	if r.Preset == "" {
		if r.Selector == nil {
			r.Selector = map[string]any{}
		}
		return []item{{r.Capability, r.Selector, r.SandboxLevel}}, r.Duration, nil
	}
	var raw, durRaw []byte
	if err := w.Row(`SELECT items,duration FROM grant_presets WHERE id=$1`, r.Preset).Scan(&raw, &durRaw); err != nil {
		return nil, "", store.ErrNotFound("grant preset " + r.Preset)
	}
	var items []item
	var dur string
	_ = json.Unmarshal(raw, &items)
	_ = json.Unmarshal(durRaw, &dur)
	if r.Duration != "" {
		if !scope.DurationWithin(r.Duration, dur) {
			return nil, "", store.Fail(422, "scope_wider_than_preset", "duration %s is longer than the preset's %s", r.Duration, dur)
		}
		dur = r.Duration
	}
	for i := range items {
		if items[i].Selector == nil {
			items[i].Selector = map[string]any{}
		}
	}
	return items, dur, nil
}

func checkDuration(w *store.W, r Req, dur string) error {
	set, _ := w.Settings()
	ok := false
	for _, d := range set.Strings("grant_duration_options") {
		ok = ok || d == dur
	}
	if !ok {
		return store.ErrInvalid("duration %q is not one of grant_duration_options", dur)
	}
	if dur == "task" && r.TaskID == nil && r.GuideRunID == "" {
		return store.ErrInvalid("duration task needs task_id or guide_run_id")
	}
	if dur == "until_revoked" {
		if err := w.Can("grant.indefinite"); err != nil {
			return store.Fail(403, "policy_denied", "until_revoked needs policy grant.indefinite for %s", w.P.ID)
		}
	}
	return nil
}
