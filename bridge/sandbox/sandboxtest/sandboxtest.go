// Package sandboxtest gates tests that need a working OS sandbox (bubblewrap on Linux, Seatbelt on macOS).
//
// Where no sandbox works (bwrap missing, or user namespaces blocked as on stock Ubuntu 24.04 runners) the
// test is skipped with the reason, so a developer laptop stays green. CI sets DEVAGOTCHI_REQUIRE_SANDBOX=1,
// which turns that skip into a failure: the gate never passes by quietly skipping the confinement proofs.
package sandboxtest

import (
	"os"
	"sync"
	"testing"

	"devagotchi/bridge/sandbox"
)

// RequireEnv is the variable that makes a missing sandbox a test failure instead of a skip.
const RequireEnv = "DEVAGOTCHI_REQUIRE_SANDBOX"

var (
	once sync.Once
	kind string
)

// Kind is sandbox.Kind, probed once per test binary (the probe runs a real confined /bin/true).
func Kind() string {
	once.Do(func() { kind = sandbox.Kind() })
	return kind
}

// Available reports whether a working OS sandbox exists here.
func Available() bool { return Kind() != "none" }

// Require skips t when no OS sandbox works here, or fails it when RequireEnv=1.
func Require(t testing.TB) {
	t.Helper()
	if Available() {
		return
	}
	const why = "no working OS sandbox here (bwrap missing or unprivileged user namespaces blocked; run `devagotchi doctor`)"
	if os.Getenv(RequireEnv) == "1" {
		t.Fatalf("%s, and %s=1 requires one", why, RequireEnv)
	}
	t.Skipf("%s; set %s=1 to make this a failure", why, RequireEnv)
}
