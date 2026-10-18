package scope

import (
	"encoding/json"
	"testing"
)

func sel(t *testing.T, s string) Sel {
	var m Sel
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestNarrowing(t *testing.T) {
	r := `{"paths":["/home/a/code"],"argv_prefix":[["npm"],["git"]],"hosts":["*.npmjs.org"],"max_runtime_s":600}`
	ok := []string{
		r,
		`{"paths":["/home/a/code/shop"],"argv_prefix":[["npm","test"]],"hosts":["registry.npmjs.org"],"max_runtime_s":60}`,
		`{"paths":["/home/a/code"],"argv_prefix":[["git"]],"hosts":[],"max_runtime_s":600}`,
		`{"paths":["/home/a/code"],"argv_prefix":[["git"]],"max_runtime_s":600}`,
	}
	bad := []string{
		`{"paths":["/home/a"],"argv_prefix":[["git"]],"max_runtime_s":600}`,
		`{"paths":["/home/a/code"],"argv_prefix":[["bash"]],"max_runtime_s":600}`,
		`{"paths":["/home/a/code"],"argv_prefix":[["git"]],"hosts":["*"],"max_runtime_s":600}`,
		`{"paths":["/home/a/code"],"argv_prefix":[["git"]],"max_runtime_s":601}`,
		`{"argv_prefix":[["git"]],"max_runtime_s":600}`,
		`{"paths":["/home/a/code"],"argv_prefix":[["git"]]}`,
		`{"paths":["/home/a/codex"],"argv_prefix":[["git"]],"max_runtime_s":600}`,
	}
	for _, g := range ok {
		if err := SelectorWithin(sel(t, g), sel(t, r)); err != nil {
			t.Errorf("should be within: %s: %v", g, err)
		}
	}
	for _, g := range bad {
		if SelectorWithin(sel(t, g), sel(t, r)) == nil {
			t.Errorf("should be wider: %s", g)
		}
	}
	if SelectorWithin(sel(t, `{"env_allow":["HOME"]}`), sel(t, `{}`)) == nil {
		t.Error("env_allow out of nothing")
	}
	if SelectorWithin(sel(t, `{"apps":"any"}`), sel(t, `{"apps":["gedit"]}`)) == nil {
		t.Error("apps widened to any")
	}
	if SelectorWithin(sel(t, `{"deny_keys":[]}`), sel(t, `{"deny_keys":["super"]}`)) == nil {
		t.Error("deny key dropped")
	}
}

func TestLevelsAndDurations(t *testing.T) {
	for _, c := range [][2]string{{"read-only", "workspace-write"}, {"workspace-write", "unsandboxed"}, {"desktop", "unsandboxed"}} {
		if !LevelWithin(c[0], c[1]) || LevelWithin(c[1], c[0]) {
			t.Errorf("level %v", c)
		}
	}
	if LevelWithin("desktop", "workspace-write") {
		t.Error("desktop is not a filesystem level")
	}
	if !DurationWithin("1h", "8h") || DurationWithin("8h", "1h") || DurationWithin("8h", "task") || !DurationWithin("once", "task") || !DurationWithin("task", "until_revoked") {
		t.Error("durations")
	}
}

func TestFits(t *testing.T) {
	s := sel(t, `{"paths":["/w"],"argv_prefix":[["podman"]],"hosts":["quay.io","*.quay.io"]}`)
	if err := Fits("exec", "workspace-write", s, Call{Op: "exec", Argv: []string{"podman", "run"}, Paths: []string{"/w/x"}, Hosts: []string{"cdn01.quay.io"}}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []Call{
		{Op: "exec", Argv: []string{"podmanx"}},
		{Op: "exec", Argv: []string{"sh", "-c", "podman run"}},
		{Op: "exec", Argv: []string{"podman"}, Paths: []string{"/etc"}},
		{Op: "exec", Argv: []string{"podman"}, Hosts: []string{"evil.io"}},
		{Op: "exec", Argv: []string{"podman"}, Level: "unsandboxed"},
		{Op: "fs.write", Paths: []string{"/w/a"}},
	} {
		if Fits("exec", "workspace-write", s, c) == nil {
			t.Errorf("should not fit: %+v", c)
		}
	}
	if Fits("fs.write", "workspace-write", sel(t, `{}`), Call{Op: "fs.read", Paths: []string{"/x"}}) != nil {
		t.Error("write grant should cover reads")
	}
}

func TestRiskWhen(t *testing.T) {
	s := sel(t, `{"argv_prefix":[["docker"]],"hosts":["*.docker.io"]}`)
	if !RiskWhen(sel(t, `{"argv0":["docker","podman"]}`), s, "workspace-write", "task", "", "native") {
		t.Error("argv0")
	}
	if !RiskWhen(sel(t, `{"hosts_wildcard":true}`), s, "", "", "", "") || RiskWhen(sel(t, `{"hosts_wildcard":true}`), sel(t, `{"hosts":["a.io"]}`), "", "", "", "") {
		t.Error("wildcard")
	}
	if !RiskWhen(sel(t, `{"path_prefix":["/mnt/"]}`), sel(t, `{"paths":["/mnt/c/x"]}`), "", "", "", "wsl") {
		t.Error("path prefix")
	}
	if RiskWhen(sel(t, `{"unknown":1}`), s, "", "", "", "") {
		t.Error("unknown keys never match")
	}
}
