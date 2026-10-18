package hubtest

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"testing"

	"agenthub/internal/guidekit"
)

func TestGuidesEditPublishImportRun(t *testing.T) {
	e := setup(t)
	if l := e.callList("claude-code", "/guides"); len(l) < 10 {
		t.Fatalf("seed guides: %d", len(l))
	}
	g := e.call("jesse", "GET", "/guides/podman-setup", nil)
	steps := g.Body["steps"]
	draft := map[string]any{"semver": "1.0.1", "change_notes": "Local wording", "steps": steps, "applies_to": map[string]any{"os": []string{"linux", "macos", "windows"}}}
	if r := e.call("claude-code", "POST", "/guides/podman-setup/versions", draft); r.code() != "humans_only" {
		t.Fatalf("agent edited a guide: %v", r.Body)
	}
	r := e.call("jesse", "POST", "/guides/podman-setup/versions", draft)
	if r.Body["guide_id"] != "local-podman-setup" {
		t.Fatalf("editing a Shippin guide must create a tenant override: %v", r.Body)
	}
	vid := r.Body["version_id"].(string)
	if r := e.call("claude-code", "POST", "/guides/local-podman-setup/versions/"+vid+"/publish", nil); r.code() != "humans_only" {
		t.Fatalf("agent published: %v", r.Body)
	}
	if r := e.call("jesse", "POST", "/guides/local-podman-setup/versions/"+vid+"/publish", nil); r.Status != 200 {
		t.Fatalf("publish: %v", r.Body)
	}
	cs := e.mcpSession("claude-code")
	if out, _ := callTool(t, cs, "guide_get", map[string]any{"guide": "podman-setup"}); out["id"] != "local-podman-setup" {
		t.Fatalf("override not effective: %v", out["id"])
	}
	bad := []any{map[string]any{"id": "a", "kind": "explain", "title": "hello", "text": "paste api_key=abcdefghijkl here"}}
	r = e.call("jesse", "POST", "/guides", map[string]any{"guide_id": "mine", "title": "Mine", "semver": "1.0.0", "steps": bad})
	if r.Body["blocking"] != true {
		t.Fatalf("secret lint: %v", r.Body)
	}
	if p := e.call("jesse", "POST", "/guides/mine/versions/"+r.Body["version_id"].(string)+"/publish", nil); p.code() != "lint_blocked" {
		t.Fatalf("published with a blocking lint: %v", p.Body)
	}
	out, isErr := callTool(t, cs, "guide_run_start", map[string]any{"guide": "docker-setup", "dry_run": true, "profile": map[string]any{"os": "linux", "kind": "native"}})
	if isErr || out["step"] == nil {
		t.Fatalf("dry run start: %v", out)
	}
	pub, priv, _ := ed25519.GenerateKey(nil)
	b := guidekit.Bundle{GuideID: "podman-setup", Semver: "1.1.0", Title: "Set up Podman", Goal: "g", Tags: []string{"podman"}, AppliesTo: json.RawMessage(`{}`),
		ChangeNotes: "Shippin update", Steps: mustJSON(steps), KeyID: "shippin-test"}
	signed, _ := guidekit.Sign(b, priv)
	if r := e.call("jesse", "POST", "/guides/import", signed); r.code() != "bad_signature" {
		t.Fatalf("untrusted key accepted: %v", r.Body)
	}
	e.setSetting("guide_signing_keys", []map[string]string{{"id": "shippin-test", "key": base64.StdEncoding.EncodeToString(pub)}})
	tampered := signed
	tampered.ChangeNotes = "evil"
	if r := e.call("jesse", "POST", "/guides/import", tampered); r.code() != "bad_signature" {
		t.Fatalf("tampered bundle accepted: %v", r.Body)
	}
	r = e.call("jesse", "POST", "/guides/import", signed)
	if r.Body["queued"] == nil {
		t.Fatalf("update should queue (notify mode): %v", r.Body)
	}
	if d := e.call("jesse", "POST", "/guide-updates/"+r.Body["queued"].(string)+"/merge", nil); d.Body["draft_version_id"] == nil {
		t.Fatalf("merge into override: %v", d.Body)
	}
	ev := e.call("jesse", "GET", "/events?action=guide.import_rejected", nil)
	if l, _ := ev.Body["events"].([]any); len(l) < 2 {
		t.Logf("events body: %v", ev.Body)
	}
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
