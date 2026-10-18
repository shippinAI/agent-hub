package tools

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"devagotchi/bridge/netproxy"
	"devagotchi/bridge/policy"
	"devagotchi/bridge/sandbox"
)

// confined builds a sandboxed command for argv in cwd. stop releases the egress proxy (call after exit).
func (e *Env) confined(argv []string, cwd string, pty bool) (*exec.Cmd, func(), error) {
	if len(argv) == 0 || argv[0] == "" {
		return nil, nil, &policy.Denied{Code: "invalid", Msg: "empty command"}
	}
	level := e.Gate.Level()
	if level != "unsandboxed" && sandbox.Kind() == "none" {
		return nil, nil, &policy.Denied{Code: "sandbox_unavailable", Msg: "no OS sandbox on this machine (run `devagotchi doctor`); only a policy with sandbox_level=unsandboxed may run processes here"}
	}
	rw, ro := e.Gate.Roots()
	if cwd == "" {
		cwd = first(rw, ro)
	} else if r, err := e.Gate.Path(policy.CapRead, cwd); err != nil {
		return nil, nil, err
	} else {
		cwd = r
	}
	env := map[string]string{}
	for _, k := range e.Gate.P.EnvAllow {
		if v, ok := lookupEnv(k); ok {
			env[k] = v
		}
	}
	if pty {
		env["TERM"] = "xterm-256color"
	}
	spec := sandbox.Spec{Level: level, Paths: rw, ReadPaths: ro, Cwd: cwd, Argv: argv, Env: env, Self: e.Self}
	sock, tcp, stop := netproxy.Start(context.Background(), e.Gate.P.EgressHosts, e.Log)
	spec.ProxySock, spec.ProxyTCP = sock, tcp
	cmd, err := sandbox.Command(spec)
	if err != nil {
		stop()
		return nil, nil, &policy.Denied{Code: "sandbox_unavailable", Msg: err.Error()}
	}
	return cmd, stop, nil
}

func first(lists ...[]string) string {
	for _, l := range lists {
		if len(l) > 0 {
			return l[0]
		}
	}
	return ""
}

// argvFor turns (argv | command + shell profile) into argv, and reports whether the profile wants a PTY.
func argvFor(p policy.Policy, argv []string, command, shell string) ([]string, bool, error) {
	if len(argv) > 0 {
		return argv, false, nil
	}
	if strings.TrimSpace(command) == "" {
		return nil, false, fmt.Errorf("give argv (preferred, no shell) or command")
	}
	prof, err := profile(p.ShellProfiles, shell, p.Shell)
	if err != nil {
		return nil, false, err
	}
	return append(append([]string{}, prof.Argv...), command), prof.PTY, nil
}

// profile finds a shell/runner profile by name, or by the base name of a shell path ("/bin/zsh" -> zsh).
func profile(list []policy.ShellProfile, name, fallback string) (policy.ShellProfile, error) {
	if name == "" {
		name = fallback
	}
	base := strings.TrimSuffix(filepath.Base(name), ".exe")
	var names []string
	for _, s := range list {
		if s.Name == name || s.Name == base || len(s.Argv) > 0 && strings.TrimSuffix(filepath.Base(s.Argv[0]), ".exe") == base {
			return s, nil
		}
		names = append(names, s.Name)
	}
	return policy.ShellProfile{}, fmt.Errorf("no profile %q in the policy (available: %s); a human adds profiles to policy.toml", name, strings.Join(names, ", "))
}
