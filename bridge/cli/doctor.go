package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"devagotchi/bridge/buildinfo"
	"devagotchi/bridge/config"
	"devagotchi/bridge/mcpserve"
	"devagotchi/bridge/policy"
	"devagotchi/bridge/procs"
	"devagotchi/bridge/sandbox"
)

type check struct{ level, name, detail string } // level: ok | warn | fail

// cmdDoctor runs local self-checks and exits non-zero if any check fails. It changes nothing.
func cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	var pf policyFlags
	pf.bind(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	var cs []check
	add := func(l, n, f string, a ...any) { cs = append(cs, check{l, n, fmt.Sprintf(f, a...)}) }
	add("ok", "build", "%s", buildinfo.String())
	if !map[string]bool{"linux": true, "darwin": true, "windows": true}[runtime.GOOS] {
		add("fail", "os", "%s is not a supported OS (linux, darwin, windows)", runtime.GOOS)
	}
	p, found, err := pf.load()
	switch {
	case err != nil:
		add("fail", "policy", "%s: %v", pf.file(), err)
	case !found:
		add("warn", "policy", "no %s yet; defaults apply (`devagotchi config init --root DIR`)", pf.file())
	default:
		add("ok", "policy", "%s: level %s, %d roots, %d read-only roots, %d grants", pf.file(), p.SandboxLevel, len(p.Roots), len(p.ReadOnlyRoots), len(p.Grants))
	}
	for _, r := range append(append([]string{}, p.Roots...), p.ReadOnlyRoots...) {
		if _, err := os.Stat(policy.Resolve(r)); err != nil {
			add("warn", "root", "%s does not exist", r)
		}
	}
	add(permLevel(pf.dir), "config-dir", "%s %s", pf.dir, permNote(pf.dir))
	if fi, err := os.Stat(filepath.Join(pf.dir, mcpserve.TokenFile)); err == nil && runtime.GOOS != "windows" && fi.Mode().Perm()&0o077 != 0 {
		add("fail", "http-token", "%s is readable by others (chmod 600)", fi.Name())
	}
	if k := sandbox.Kind(); k == "none" {
		add("warn", "sandbox", "no OS sandbox here: processes are refused unless sandbox_level = unsandboxed (%s)", sandboxHint())
	} else {
		cs = append(cs, sandboxCheck(k)...)
	}
	cs = append(cs, ptyCheck())
	if ps, err := procs.ListAll(); err != nil {
		add("warn", "process-list", "%v", err)
	} else {
		add("ok", "process-list", "%d processes visible", len(ps))
	}
	if _, err := config.Load(config.Dir()); err == nil {
		add("ok", "hub", "enrolled (bridge.toml in %s); `devagotchi connect` runs hub mode", config.Dir())
	} else {
		add("ok", "hub", "not enrolled; standalone `devagotchi mcp` works without a hub")
	}
	add("ok", "telemetry", "none: this binary only talks to a hub you enroll and URLs your policy allows")
	failed := 0
	for _, c := range cs {
		fmt.Printf("[%-4s] %-13s %s\n", c.level, c.name, c.detail)
		if c.level == "fail" {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d check(s) failed", failed)
	}
	return nil
}

func permLevel(dir string) string {
	if fi, err := os.Stat(dir); err == nil && runtime.GOOS != "windows" && fi.Mode().Perm()&0o022 != 0 {
		return "warn"
	}
	return "ok"
}

func permNote(dir string) string {
	fi, err := os.Stat(dir)
	if err != nil {
		return "(not created yet)"
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm()&0o022 != 0 {
		return fmt.Sprintf("(mode %o: others can write; chmod 700)", fi.Mode().Perm())
	}
	return "(private)"
}

func sandboxHint() string {
	switch runtime.GOOS {
	case "linux":
		return "install bubblewrap, and allow unprivileged user namespaces"
	case "windows":
		return "the Windows restricted-token sandbox is not built yet"
	}
	return "sandbox-exec is missing"
}
