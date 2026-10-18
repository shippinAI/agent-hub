package actionfeed

import (
	"encoding/json"
	"strings"

	"devagotchi/bridge/redact"
)

// Scrubber masks secrets: values with a known secret prefix anywhere (bridge redact + secret_patterns), and
// every value under a sensitive key name (audit_redact_keys) inside argument objects. Flags like
// --token=... and "Authorization: Bearer ..." are masked too.
type Scrubber struct {
	r    redact.Redactor
	keys []string
}

// NewScrubber builds a scrubber from secret prefixes and sensitive key names.
func NewScrubber(prefixes, keys []string) Scrubber {
	lk := make([]string, 0, len(keys))
	for _, k := range keys {
		if k = strings.ToLower(strings.TrimSpace(k)); k != "" {
			lk = append(lk, k)
		}
	}
	return Scrubber{r: redact.New(prefixes), keys: lk}
}

func (s Scrubber) sensitive(key string) bool {
	k := strings.ReplaceAll(strings.ToLower(strings.TrimLeft(key, "-")), "-", "_")
	for _, x := range s.keys {
		if strings.Contains(k, x) {
			return true
		}
	}
	return false
}

// Text masks one string and reports how many masks it applied.
func (s Scrubber) Text(v string) (string, int) {
	out := s.r.String(v)
	n := strings.Count(out, "[redacted]") - strings.Count(v, "[redacted]")
	low := strings.ToLower(out)
	if i := strings.Index(low, "bearer "); i >= 0 && !strings.HasPrefix(out[i+7:], "[redacted]") {
		end := i + 7
		for end < len(out) && out[end] != ' ' && out[end] != '\n' && out[end] != '"' {
			end++
		}
		out, n = out[:i+7]+"[redacted]"+out[end:], n+1
	}
	return out, n
}

// Argv masks each element, and the value after or inside a sensitive flag (--api-key X, --token=X).
func (s Scrubber) Argv(argv []string) ([]string, int) {
	out, n := make([]string, len(argv)), 0
	for i, a := range argv {
		if k, _, ok := strings.Cut(a, "="); ok && strings.HasPrefix(k, "-") && s.sensitive(k) {
			out[i], n = k+"=[redacted]", n+1
			continue
		}
		if i > 0 && strings.HasPrefix(argv[i-1], "-") && !strings.Contains(argv[i-1], "=") && s.sensitive(argv[i-1]) {
			out[i], n = "[redacted]", n+1
			continue
		}
		var c int
		out[i], c = s.Text(a)
		n += c
	}
	return out, n
}

// JSON masks values under sensitive keys and secret-looking strings at any depth.
func (s Scrubber) JSON(raw json.RawMessage) (json.RawMessage, int) {
	if len(raw) == 0 {
		return raw, 0
	}
	var v any
	if json.Unmarshal(raw, &v) != nil {
		t, n := s.Text(string(raw))
		b, _ := json.Marshal(t)
		return b, n
	}
	n := 0
	v = s.walk(v, &n)
	b, _ := json.Marshal(v)
	return b, n
}

func (s Scrubber) walk(v any, n *int) any {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if str, isStr := val.(string); isStr && str != "" && s.sensitive(k) { // numbers (max_tokens) stay
				x[k] = "[redacted]"
				*n++
				continue
			}
			x[k] = s.walk(val, n)
		}
	case []any:
		for i := range x {
			x[i] = s.walk(x[i], n)
		}
	case string:
		t, c := s.Text(x)
		*n += c
		return t
	}
	return v
}
