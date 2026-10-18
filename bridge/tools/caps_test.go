package tools

import (
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
