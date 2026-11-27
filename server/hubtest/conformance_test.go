package hubtest

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"agenthub/conformance"
)

// TestConformance runs the black-box protocol suite against the Go server, the same suite another build runs.
func TestConformance(t *testing.T) {
	e := setup(t)
	tg := conformance.Target{URL: e.url, Human: e.tokens["jesse"], AgentA: e.tokens["codex"], AgentB: e.tokens["claude-code"]}
	conformance.Run(t, tg)

	// The external entry point another build uses: the suite as its own test binary, given only a URL and tokens.
	t.Run("external", func(t *testing.T) {
		if testing.Short() {
			t.Skip("short")
		}
		f := filepath.Join(t.TempDir(), "tokens.json")
		b, _ := json.Marshal(tg)
		if err := os.WriteFile(f, b, 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("go", "test", "-count=1", "-v", "-run", "TestExternal", "agenthub/conformance")
		cmd.Env = append(os.Environ(), "HUB_CONFORMANCE_URL="+e.url, "HUB_CONFORMANCE_TOKENS="+f)
		out, err := cmd.CombinedOutput()
		if err != nil || !strings.Contains(string(out), "--- PASS: TestExternal") || strings.Contains(string(out), "SKIP") {
			t.Fatalf("external run: %v\n%s", err, out)
		}
	})
}
