// Package sandbox turns a grant scope into an OS-confined command: bubblewrap on Linux, Seatbelt on macOS,
// a job object with a restricted token on Windows. The scope comes from the signed grant, never call args.
package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Spec is one confined run.
type Spec struct {
	Level      string            // read-only | workspace-write | unsandboxed
	Paths      []string          // selector paths (read for read-only, read+write for workspace-write)
	ReadPaths  []string          // extra paths that are always read-only (policy read_only_roots)
	Cwd        string            // working directory (must sit inside Paths when Paths is set)
	Argv       []string          // command
	Env        map[string]string // already filtered by env_allow
	ProxySock  string            // unix socket of the CONNECT allowlist proxy ("" = no network; Linux)
	ProxyTCP   string            // loopback address of the same proxy (macOS/Windows, where no netns exists)
	UsernsRoot bool              // map the caller to uid 0 inside a user namespace (rootless container runtimes)
	Self       string            // path of the bridge binary (runs as the in-sandbox init)
}

// Kind reports which sandbox this OS build uses ("none" when its self-test fails).
func Kind() string { return kind() }

// Command builds the confined command. Unsandboxed runs as the bridge user with no OS sandbox (never elevated).
func Command(s Spec) (*exec.Cmd, error) {
	s.Paths = expandAll(s.Paths)
	s.ReadPaths = expandAll(s.ReadPaths)
	s.Cwd = expand(s.Cwd)
	if s.Cwd == "" && len(s.Paths) > 0 {
		s.Cwd = s.Paths[0]
	}
	if s.Level == "unsandboxed" {
		c := exec.Command(s.Argv[0], s.Argv[1:]...)
		c.Dir, c.Env = s.Cwd, envList(s.Env, os.Getenv("HOME"))
		detach(c)
		return c, nil
	}
	return build(s)
}

func expand(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		p = filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	if p == "" {
		return ""
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

func expandAll(ps []string) []string {
	out := []string{}
	for _, p := range ps {
		if e := expand(p); e != "" {
			out = append(out, e)
		}
	}
	return out
}

func envList(env map[string]string, home string) []string {
	out := []string{"PATH=" + firstNonEmpty(os.Getenv("PATH"), "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"), "HOME=" + home, "LANG=C.UTF-8"}
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
