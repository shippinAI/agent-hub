package runner

import (
	"context"
	"devagotchi/bridge/fsops"
	"devagotchi/bridge/scope"
	"devagotchi/bridge/wire"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func (r *Runner) fs(ctx context.Context, f wire.Frame) wire.Result {
	var op fsops.Op
	_ = json.Unmarshal(f.Args, &op)
	op.Kind = strings.TrimPrefix(f.Op, "fs.")
	sel := scope.Sel(f.Grant.Selector)
	paths := strsOf(sel, "paths")
	for _, p := range append([]string{op.Path}, op.To) {
		if p != "" && !pathAllowed(p, paths) {
			return fail("grant_required", fmt.Sprintf("path %s is outside the grant", p))
		}
	}
	res := fsops.DoSandboxed(ctx, r.Self, f.Grant.Level, paths, op)
	if res.Error != "" {
		return wire.Result{OK: false, Error: res.Error, Message: res.Message, Data: fsops.Marshal(res)}
	}
	return wire.Result{OK: true, Data: fsops.Marshal(res), OutSHA256: res.SHA256}
}

func pathAllowed(p string, paths []string) bool {
	if len(paths) == 0 {
		return true
	}
	for _, a := range paths {
		if scope.Within(p, a) {
			return true
		}
	}
	return false
}

func filterEnv(env map[string]string, sel scope.Sel) map[string]string {
	allow, ok := sel.Strs("env_allow")
	if !ok {
		return map[string]string{}
	}
	out := map[string]string{}
	for _, k := range allow {
		if v, ok := env[k]; ok {
			out[k] = v
		} else if v := os.Getenv(k); v != "" {
			out[k] = v
		}
	}
	return out
}

func strsOf(sel scope.Sel, k string) []string {
	s, _ := sel.Strs(k)
	return s
}
