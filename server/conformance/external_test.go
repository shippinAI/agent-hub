package conformance

import (
	"encoding/json"
	"os"
	"testing"
)

// TestExternal runs the suite against any running hub:
//
//	HUB_CONFORMANCE_URL=http://127.0.0.1:8080 HUB_CONFORMANCE_TOKENS=tokens.json go test ./conformance
//
// tokens.json is {"human": "...", "agent_a": "...", "agent_b": "..."}.
func TestExternal(t *testing.T) {
	url, file := os.Getenv("HUB_CONFORMANCE_URL"), os.Getenv("HUB_CONFORMANCE_TOKENS")
	if url == "" || file == "" {
		t.Skip("HUB_CONFORMANCE_URL and HUB_CONFORMANCE_TOKENS not set")
	}
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var tg Target
	if err := json.Unmarshal(b, &tg); err != nil {
		t.Fatal(err)
	}
	tg.URL = url
	Run(t, tg)
}
