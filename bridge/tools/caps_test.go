package tools

import (
	"encoding/json"
	"testing"

	"devagotchi/bridge/toolcaps"
)

func TestCapsTable(t *testing.T) {
	if len(All()) != len(toolcaps.Cap) {
		t.Fatalf("toolcaps has %d entries, tools.All has %d", len(toolcaps.Cap), len(All()))
	}
	for _, tl := range All() {
		if c, ok := toolcaps.Cap[tl.Name]; !ok || c != tl.Cap {
			t.Errorf("%s: toolcaps %q, tool %q", tl.Name, c, tl.Cap)
		}
	}
}

// TestSchemasAreObjects: TypeScript clients (Claude Desktop/Code, Cursor) reject boolean sub-schemas.
func TestSchemasAreObjects(t *testing.T) {
	for _, tl := range All() {
		for k, ps := range tl.Schema.Properties {
			if b, _ := json.Marshal(ps); len(b) == 0 || b[0] != '{' {
				t.Errorf("%s.%s schema is %s", tl.Name, k, b)
			}
		}
	}
}
