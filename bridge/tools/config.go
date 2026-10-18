package tools

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"devagotchi/bridge/buildinfo"
	"devagotchi/bridge/policy"
	"devagotchi/bridge/sandbox"
)

func getConfig() Tool {
	return def(Tool{Name: "get_config", Title: "Get Configuration", ReadOnly: true,
		Desc: "Show the effective policy (sandbox level, roots, read-only roots, capabilities, live grants, limits, shell " +
			"and code-runner profiles, URL/egress allowlists), the sandbox in use, version, OS, client and config path. " +
			"Secrets are never included. There is no telemetry setting because there is no telemetry."},
		func(ctx context.Context, e *Env, _ noIn) Out {
			p := e.Gate.P
			rw, ro := e.Gate.Roots()
			var grants []map[string]any
			for _, g := range p.Grants {
				if g.Live(e.Gate.Now()) {
					grants = append(grants, map[string]any{"id": g.ID, "capability": g.Capability, "paths": g.Paths, "hosts": g.Hosts, "expires": g.Expires})
				}
			}
			host, _ := os.Hostname()
			cfg := map[string]any{
				"version": buildinfo.Version, "commit": buildinfo.CommitOrVCS(), "mode": e.Mode, "config_path": e.ConfigPath,
				"sandbox": map[string]any{"kind": sandbox.Kind(), "level": e.Gate.Level()},
				"roots": rw, "read_only_roots": ro, "client_roots_narrowing": e.Gate.Narrowed(),
				"capabilities": p.Capabilities, "grants": grants, "limits": e.Gate.Limits(),
				"shell": p.Shell, "shell_profiles": p.ShellProfiles, "code_runners": p.CodeRunners,
				"url_hosts": p.URLHosts, "egress_hosts": p.EgressHosts, "env_allow": p.EnvAllow, "history": p.History,
				"telemetry": false, "client": e.Client,
				"system": map[string]any{"os": runtime.GOOS, "arch": runtime.GOARCH, "cpus": runtime.NumCPU(), "hostname": host,
					"pid": os.Getpid(), "go": runtime.Version()},
			}
			return ok(render(cfg), cfg)
		})
}

type setConfigIn struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

func setConfigValue() Tool {
	return def(Tool{Name: "set_config_value", Title: "Set Configuration Value", Destructive: true,
		Desc: "Kept for compatibility. The model cannot change its own policy (DC lets it rewrite allowedDirectories and " +
			"blockedCommands); this always returns policy_denied with the exact command a human can run instead."},
		func(ctx context.Context, e *Env, in setConfigIn) Out {
			v := fmt.Sprint(in.Value)
			if b, err := jsonText(in.Value); err == nil {
				v = b
			}
			return errOut(&policy.Denied{Code: "policy_denied", Msg: fmt.Sprintf(
				"the model cannot change its own configuration; a human can run: devagotchi config set %s '%s'", in.Key, v)})
		})
}
