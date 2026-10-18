// Package buildinfo holds the version stamped in at link time (-X) by scripts/release.sh.
package buildinfo

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// Set by -ldflags "-X devagotchi/bridge/buildinfo.Version=... -X ...Commit=... -X ...Date=...".
var (
	Version = "0.2.0-dev"
	Commit  = ""
	Date    = ""
)

// CommitOrVCS returns Commit, falling back to the VCS revision Go embeds when built from a checkout.
func CommitOrVCS() string {
	if Commit != "" {
		return Commit
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 12 {
				return s.Value[:12]
			}
		}
	}
	return "unknown"
}

// String is the one-line version report.
func String() string {
	return fmt.Sprintf("devagotchi %s (commit %s, built %s, %s/%s, %s)", Version, CommitOrVCS(), orNA(Date), runtime.GOOS, runtime.GOARCH, runtime.Version())
}

func orNA(s string) string {
	if s == "" {
		return "n/a"
	}
	return s
}
