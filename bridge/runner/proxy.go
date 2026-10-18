package runner

import (
	"context"

	"devagotchi/bridge/netproxy"
	"devagotchi/bridge/sandbox"
	"devagotchi/bridge/scope"
)

// startProxy starts the egress allowlist for the grant's hosts and points the spec at it.
// No hosts in the selector means no network at all.
func (r *Runner) startProxy(ctx context.Context, spec *sandbox.Spec, sel scope.Sel) func() {
	hosts, _ := sel.Strs("hosts")
	sock, tcp, stop := netproxy.Start(ctx, hosts, r.Log)
	spec.ProxySock, spec.ProxyTCP = sock, tcp
	return stop
}
