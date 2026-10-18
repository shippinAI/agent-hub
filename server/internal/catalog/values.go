package catalog

import (
	"encoding/json"
	"slices"

	"agenthub/internal/schemacheck"
	"agenthub/internal/store"
)

// values converts the present body fields into SQL args by column type.
func values(k Kind, body map[string]json.RawMessage) ([]string, []any, error) {
	var names []string
	var args []any
	for _, c := range k.Cols {
		raw, ok := body[c.name]
		if !ok {
			continue
		}
		var v any
		var err error
		switch c.typ {
		case tText:
			var s string
			err, v = json.Unmarshal(raw, &s), s
		case tTextNull:
			var s *string
			err, v = json.Unmarshal(raw, &s), s
		case tTextArr:
			s := []string{}
			if string(raw) != "null" {
				err = json.Unmarshal(raw, &s)
			}
			v = s
		case tJSON:
			var probe any
			err, v = json.Unmarshal(raw, &probe), []byte(raw)
		case tBool:
			var b bool
			err, v = json.Unmarshal(raw, &b), b
		case tInt:
			var n int64
			err, v = json.Unmarshal(raw, &n), n
		}
		if err != nil {
			return nil, nil, store.ErrInvalid("field %s: %v", c.name, err)
		}
		names, args = append(names, c.name), append(args, v)
	}
	if len(names) == 0 {
		return nil, nil, store.ErrInvalid("no known fields in body")
	}
	return names, args, nil
}

// PresetItem is one line of a grant preset.
type PresetItem struct {
	Capability   string          `json:"capability"`
	Selector     json.RawMessage `json:"selector"`
	SandboxLevel string          `json:"sandbox_level"`
}

// validate applies the per-kind rules that keep data consistent (and the full-access invariant).
func validate(w *store.W, name string, body map[string]json.RawMessage) error {
	switch name {
	case "capabilities":
		if raw, ok := body["selector_schema"]; ok {
			if _, err := schemacheck.Compile(raw); err != nil {
				return store.ErrInvalid("selector_schema: %v", err)
			}
		}
	case "grant-presets":
		var full bool
		var req *string
		_ = json.Unmarshal(body["full_access"], &full)
		_ = json.Unmarshal(body["requires_policy"], &req)
		if full && (req == nil || *req != "unsandboxed") {
			return store.ErrInvalid("a full_access preset must set requires_policy to \"unsandboxed\"")
		}
		var items []PresetItem
		if raw, ok := body["items"]; ok {
			if err := json.Unmarshal(raw, &items); err != nil {
				return store.ErrInvalid("items: %v", err)
			}
		}
		for _, it := range items {
			var levels []string
			var schema []byte
			if err := w.Row(`SELECT sandbox_levels, selector_schema FROM capabilities WHERE id=$1`, it.Capability).Scan(&levels, &schema); err != nil {
				return store.ErrInvalid("items: unknown capability %q", it.Capability)
			}
			if !slices.Contains(levels, it.SandboxLevel) {
				return store.ErrInvalid("items: %s does not offer sandbox level %q", it.Capability, it.SandboxLevel)
			}
			if err := schemacheck.Validate(schema, it.Selector); err != nil {
				return store.ErrInvalid("items: %s selector: %v", it.Capability, err)
			}
		}
	}
	return nil
}
