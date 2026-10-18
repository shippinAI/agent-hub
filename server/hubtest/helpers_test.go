package hubtest

import "encoding/json"

func (e *env) newTask(who, title string, extra map[string]any) int64 {
	body := map[string]any{"project": "general", "title": title}
	for k, v := range extra {
		body[k] = v
	}
	r := e.call(who, "POST", "/tasks", body)
	if r.Status != 200 {
		e.t.Fatalf("create task: %d %v", r.Status, r.Body)
	}
	return int64(r.Body["id"].(float64))
}

// setSetting changes a behaviour knob through the API, as Jesse (data, not code).
func (e *env) setSetting(key string, value any) {
	cur := e.call("jesse", "GET", "/config", nil)
	ver := 0.0
	for _, s := range cur.Body["settings"].([]any) {
		if m := s.(map[string]any); m["key"] == key {
			ver = m["version"].(float64)
		}
	}
	v, _ := json.Marshal(value)
	r := e.call("jesse", "PUT", "/settings/"+key, map[string]any{"value": json.RawMessage(v), "version": ver})
	if r.Status != 200 {
		e.t.Fatalf("set %s: %d %v", key, r.Status, r.Body)
	}
}

func num(v any) int64 { f, _ := v.(float64); return int64(f) }

func taskOf(r resp) map[string]any { m, _ := r.Body["task"].(map[string]any); return m }

// callList GETs a JSON array.
func (e *env) callList(who, path string) []map[string]any {
	r := e.call(who, "GET", path, nil)
	l, _ := r.Body["list"].([]any)
	out := []map[string]any{}
	for _, x := range l {
		m, _ := x.(map[string]any)
		out = append(out, m)
	}
	return out
}
