package access

import (
	"encoding/json"

	"agenthub/internal/schemacheck"
	"agenthub/internal/store"
	"devagotchi/bridge/scope"
)

// checkItem applies the code rules to one requested item: known capability at an offered level with a
// schema-valid selector; unsandboxed and gui.input apps:any only under a live unsandboxed policy (D1, D19);
// gui.input only alongside gui.observe.
func checkItem(w *store.W, machine string, it item, all []item) error {
	var levels []string
	var schema []byte
	if err := w.Row(`SELECT sandbox_levels, selector_schema FROM capabilities WHERE id=$1`, it.Capability).Scan(&levels, &schema); err != nil {
		return store.ErrNotFound("capability " + it.Capability)
	}
	offered := false
	for _, l := range levels {
		offered = offered || l == it.SandboxLevel
	}
	if !offered {
		return store.Fail(422, "bad_sandbox_level", "%s offers %v, not %q", it.Capability, levels, it.SandboxLevel)
	}
	if err := schemacheck.Validate(schema, jsonb(it.Selector)); err != nil {
		return store.Fail(422, "bad_selector", "%s selector: %v", it.Capability, err)
	}
	sel := scope.Sel(it.Selector)
	if it.SandboxLevel == "unsandboxed" || (it.Capability == "gui.input" && sel.Any("apps")) {
		live, err := policyLive(w, machine)
		if err != nil {
			return err
		}
		if !live {
			return store.Fail(403, "policy_denied", "the unsandboxed policy is off for this machine (Settings → Access)")
		}
	}
	if it.Capability == "gui.input" {
		has := false
		for _, o := range all {
			has = has || o.Capability == "gui.observe"
		}
		if !has {
			_ = w.Row(`SELECT EXISTS(SELECT 1 FROM grants WHERE machine_id=$1 AND principal_id=$2 AND capability='gui.observe' AND ended_at IS NULL)`,
				machine, w.P.ID).Scan(&has)
		}
		if !has {
			return store.Fail(422, "observe_required", "gui.input needs gui.observe (ask for both)")
		}
	}
	return nil
}

// Risk is one row of a request's risk snapshot (copied so later catalog edits do not rewrite history).
type Risk struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	PlainText  string `json:"plain_text"`
	Severity   string `json:"severity"`
	Mitigation string `json:"mitigation"`
}

func snapshot(w *store.W, it item, dur, display, kind string) ([]byte, error) {
	rows, err := w.Query(`SELECT cr.when_selector, r.id, r.title, r.plain_text, r.severity, r.mitigation FROM capability_risks cr
		JOIN risks r ON r.id=cr.risk_id WHERE cr.capability_id=$1 ORDER BY CASE r.severity WHEN 'high' THEN 0 WHEN 'med' THEN 1 ELSE 2 END, r.id`, it.Capability)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Risk{}
	seen := map[string]bool{}
	for rows.Next() {
		var when []byte
		var r Risk
		if err := rows.Scan(&when, &r.ID, &r.Title, &r.PlainText, &r.Severity, &r.Mitigation); err != nil {
			return nil, err
		}
		var ws scope.Sel
		_ = json.Unmarshal(when, &ws)
		if !seen[r.ID] && scope.RiskWhen(ws, scope.Sel(it.Selector), it.SandboxLevel, dur, display, kind) {
			seen[r.ID] = true
			out = append(out, r)
		}
	}
	return jsonb(out), rows.Err()
}
