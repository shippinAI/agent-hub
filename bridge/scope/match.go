package scope

import "fmt"

// Call is what an operation asks of a grant.
type Call struct {
	Op     string   // exec, proc.start, fs.read, fs.write, net, gui.observe, gui.input, gui.window, gui.app
	Argv   []string // exec / proc
	Paths  []string // cwd for exec, target paths for fs
	Hosts  []string // hosts the call says it needs (the proxy still enforces per connection)
	App    string   // gui target app
	Action string   // gui input action kind
	Level  string   // requested sandbox level ("" = the grant's)
	Ticket string   // change ticket the call runs under (hub policy gate only; the bridge ignores it)
}

// CapabilityFor maps an op to the capability that covers it.
func CapabilityFor(op string) string {
	switch op {
	case "exec":
		return "exec"
	case "proc.start", "proc.read", "proc.write", "proc.kill", "proc.heartbeat":
		return "proc.interactive"
	case "fs.read", "fs.list", "fs.stat":
		return "fs.read"
	case "fs.write", "fs.edit", "fs.move", "fs.delete":
		return "fs.write"
	case "gui.screenshot", "gui.tree", "gui.session":
		return "gui.observe"
	}
	return op
}

// Fits reports whether a call fits a grant's capability, level and selector (nil = allowed).
func Fits(capability, level string, sel Sel, c Call) error {
	if CapabilityFor(c.Op) != capability && !(capability == "fs.write" && CapabilityFor(c.Op) == "fs.read") {
		return fmt.Errorf("grant covers %s, not %s", capability, c.Op)
	}
	if c.Level != "" && !LevelWithin(c.Level, level) {
		return fmt.Errorf("sandbox level %s is wider than the grant's %s", c.Level, level)
	}
	if pre := sel.Prefixes(); len(c.Argv) > 0 && len(pre) > 0 && !anyPrefix(c.Argv, pre) {
		return fmt.Errorf("command %q is outside the granted prefixes", c.Argv[0])
	}
	if paths, ok := sel.Strs("paths"); ok && len(paths) > 0 {
		for _, p := range c.Paths {
			if !listHas("paths", paths, p) {
				return fmt.Errorf("path %s is outside the granted paths", p)
			}
		}
	}
	hosts, _ := sel.Strs("hosts")
	for _, h := range c.Hosts {
		if !listHas("hosts", hosts, h) {
			return fmt.Errorf("host %s is not granted", h)
		}
	}
	if c.App != "" && !sel.Any("apps") {
		if apps, ok := sel.Strs("apps"); ok && !contains(apps, c.App) {
			return fmt.Errorf("app %s is not granted", c.App)
		}
	}
	if acts, ok := sel.Strs("actions"); ok && len(acts) > 0 && c.Action != "" && !contains(acts, c.Action) {
		return fmt.Errorf("gui action %s is not granted", c.Action)
	}
	return nil
}

// RiskWhen reports whether a capability_risks.when_selector matches a request. Keys are data; the
// meanings are: hosts_wildcard, argv0, sandbox_level, path_prefix, display_server, apps_any, duration.
func RiskWhen(when Sel, sel Sel, level, duration, displayServer, machineKind string) bool {
	for k, v := range when {
		want := fmt.Sprint(v)
		var ok bool
		switch k {
		case "hosts_wildcard":
			hosts, _ := sel.Strs("hosts")
			wild := false
			for _, h := range hosts {
				wild = wild || h == "*" || len(h) > 1 && h[:2] == "*."
			}
			ok = fmt.Sprint(wild) == want
		case "argv0":
			l, _ := when.Strs(k)
			pre := sel.Prefixes()
			ok = len(pre) == 0
			for _, p := range pre {
				ok = ok || len(p) > 0 && contains(l, p[0])
			}
		case "sandbox_level":
			ok = level == want
		case "duration":
			ok = duration == want
		case "display_server":
			ok = displayServer == want
		case "apps_any":
			ok = fmt.Sprint(sel.Any("apps")) == want
		case "path_prefix":
			l, _ := when.Strs(k)
			paths, _ := sel.Strs("paths")
			ok = len(paths) == 0 && machineKind == "wsl"
			for _, p := range paths {
				for _, pre := range l {
					ok = ok || len(pre) > 1 && Within(p, pre)
				}
			}
		default:
			ok = false
		}
		if !ok {
			return false
		}
	}
	return true
}
