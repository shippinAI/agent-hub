package catalog

import (
	"encoding/json"
	"fmt"
	"strings"

	"agenthub/internal/policy"
	"agenthub/internal/store"
)

// marked reports whether a preset file was already applied, and marks it if not. Presets apply once,
// so rows an operator deleted do not come back.
func marked(w *store.W, name string) (bool, error) {
	tag, err := w.Exec(`INSERT INTO preset_marks(name) VALUES($1) ON CONFLICT DO NOTHING`, name)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 0, nil
}

// SeedRows inserts one preset file's rows (never overwriting) the first time it is seen.
func SeedRows(w *store.W, file, kind string, rows []map[string]json.RawMessage) (int, error) {
	if done, err := marked(w, "devagotchi/"+file); err != nil || done {
		return 0, err
	}
	k, ok := kinds[kind]
	if !ok {
		return 0, fmt.Errorf("preset %s: unknown kind %s", file, kind)
	}
	n := 0
	for _, r := range rows {
		var id string
		if err := json.Unmarshal(r["id"], &id); err != nil || id == "" {
			return n, fmt.Errorf("preset %s: row without id", file)
		}
		names, args, err := values(k, r)
		if err != nil {
			return n, fmt.Errorf("preset %s/%s: %v", file, id, err)
		}
		ph := make([]string, len(names)+1)
		for i := range ph {
			ph[i] = fmt.Sprintf("$%d", i+1)
		}
		tag, err := w.Exec(`INSERT INTO `+k.Table+`(id,`+strings.Join(names, ",")+`) VALUES(`+strings.Join(ph, ",")+`) ON CONFLICT DO NOTHING`,
			append([]any{id}, args...)...)
		if err != nil {
			return n, fmt.Errorf("preset %s/%s: %w", file, id, err)
		}
		n += int(tag.RowsAffected())
	}
	if kind == "grant-presets" {
		_, err := w.Exec(`UPDATE grant_presets SET is_default = (id = (SELECT value #>> '{}' FROM settings WHERE key='grant_default_preset'))
			WHERE NOT EXISTS (SELECT 1 FROM grant_presets WHERE is_default)`)
		return n, err
	}
	return n, nil
}

// SeedPolicies adds a named policy set once (even when the hub already has other policies).
func SeedPolicies(w *store.W, set string, rules []policy.Rule) (int, error) {
	if done, err := marked(w, set); err != nil || done {
		return 0, err
	}
	for _, r := range rules {
		sel, _ := json.Marshal(r.Selector)
		if _, err := w.Exec(`INSERT INTO policies(name,subject,action,selector,effect,priority,enabled) VALUES($1,$2,$3,$4,$5,$6,true)`,
			r.Name, r.Subject, r.Action, sel, r.Effect, r.Priority); err != nil {
			return 0, err
		}
	}
	return len(rules), nil
}
