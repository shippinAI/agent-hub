package guidekit

import (
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type seedFile struct {
	ID        string `json:"id"`
	AppliesTo struct {
		OS []string `json:"os"`
	} `json:"applies_to"`
	Version struct {
		Steps json.RawMessage `json:"steps"`
	} `json:"version"`
}

func schema(t *testing.T) []byte {
	raw, err := os.ReadFile("../presets/devagotchi/step-schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Settings []struct {
			Value json.RawMessage `json:"value"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f.Settings[0].Value
}

func seeds(t *testing.T) map[string]seedFile {
	files, _ := filepath.Glob("../presets/guides/*.json")
	out := map[string]seedFile{}
	for _, p := range files {
		raw, _ := os.ReadFile(p)
		var g seedFile
		if err := json.Unmarshal(raw, &g); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		out[g.ID] = g
	}
	if len(out) < 10 {
		t.Fatalf("expected 10 seed guides, got %d", len(out))
	}
	return out
}

func TestSeedGuidesValidateAndLint(t *testing.T) {
	sc := schema(t)
	for id, g := range seeds(t) {
		steps, err := Validate(sc, g.Version.Steps)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		for _, f := range Lint(steps, g.AppliesTo.OS, nil) {
			if f.Level == "block" {
				t.Errorf("%s: %+v", id, f)
			}
		}
	}
}

func TestDockerDryRunPaths(t *testing.T) {
	g := seeds(t)["docker-setup"]
	steps, _ := Parse(g.Version.Steps)
	for _, os := range []string{"linux", "macos", "windows"} {
		path, err := DryRun(steps, map[string]any{"os": os, "kind": "native"}, nil, map[string]string{})
		if err != nil || len(path) < 3 {
			t.Fatalf("%s: %v %v", os, path, err)
		}
		t.Logf("%s: %v", os, path)
	}
}

func TestRetriesPause(t *testing.T) {
	steps := []Step{{ID: "a", Kind: "verify", Title: "aaa", Op: "exec", MaxRetries: 1}, {ID: "b", Kind: "explain", Title: "bbb"}}
	c := Ctx{}
	r, _ := Advance(steps, "a", "fail", 0, c)
	if r.Next != "a" || r.Paused {
		t.Fatalf("first fail should retry: %+v", r)
	}
	r, _ = Advance(steps, "a", "fail", 1, c)
	if !r.Paused {
		t.Fatalf("second fail should pause: %+v", r)
	}
	r, _ = Advance(steps, "a", "ok", 1, c)
	if r.Next != "b" {
		t.Fatalf("ok should go on: %+v", r)
	}
	if _, err := Advance(steps, "a", "skip", 0, c); err == nil {
		t.Fatal("verify is not skippable")
	}
}

func TestBadRefsAndSecrets(t *testing.T) {
	sc := schema(t)
	if _, err := Validate(sc, []byte(`[{"id":"a","kind":"explain","title":"hello","next":"zz"}]`)); err == nil {
		t.Fatal("unknown next accepted")
	}
	if _, err := Validate(sc, []byte(`[{"id":"a","kind":"explain","title":"hello","bogus":1}]`)); err == nil {
		t.Fatal("unknown field accepted")
	}
	f := Lint([]Step{{ID: "a", Kind: "explain", Title: "x", Text: "use api_key=abcdef123456"}}, []string{"linux"}, nil)
	if !Blocking(f) {
		t.Fatal("secret not flagged")
	}
}

func TestSignVerifyDiff(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	b := Bundle{GuideID: "g", Semver: "1.1.0", Title: "T", Steps: json.RawMessage(`[{"title":"x","id":"a","kind":"explain"}]`), KeyID: "k1"}
	s, err := Sign(b, priv)
	if err != nil {
		t.Fatal(err)
	}
	keys := []TrustedKey{{ID: "k1", PubB64: b64(pub)}}
	if err := Verify(s, keys); err != nil {
		t.Fatal(err)
	}
	s.Steps = json.RawMessage(`[{"id":"a","kind":"explain","title":"y"}]`)
	if Verify(s, keys) == nil {
		t.Fatal("tampered bundle verified")
	}
	s.KeyID = "nope"
	if Verify(s, keys) == nil {
		t.Fatal("unknown key verified")
	}
	if Bump("1.0.3", "1.1.0") != "minor" || Bump("1.1.0", "1.0.9") != "" || Bump("1.0.0", "2.0.0") != "major" {
		t.Fatal("bump")
	}
	d := Diff([]Step{{ID: "a", Title: "x"}, {ID: "b"}}, []Step{{ID: "b"}, {ID: "a", Title: "y"}, {ID: "c"}})
	if len(d) != 3 {
		t.Fatalf("diff: %+v", d)
	}
	if Fill("{{hub_url}}/x {{vars.n}} {{nope}}", map[string]any{"hub_url": "h", "vars.n": 2}) != "h/x 2 {{nope}}" {
		t.Fatal("fill")
	}
}
