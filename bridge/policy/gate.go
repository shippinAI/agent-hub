package policy

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"devagotchi/bridge/scope"
)

// Denied is a refused call. Code is grant_required (outside the scope) or policy_denied (capability off).
type Denied struct{ Code, Msg string }

func (d *Denied) Error() string { return d.Code + ": " + d.Msg }

// Gate answers "may this call happen" for one policy. Hub mode builds one from the signed grant.
type Gate struct {
	P       Policy
	AnyPath bool // hub grant without a path selector: fs paths are not limited by the bridge
	Now     func() time.Time
	roots   []string
	ro      []string
	client  []string // MCP client roots: they can only narrow
}

// NewGate resolves the policy roots (plus CLI --root extras, which a human wrote into the client config).
func NewGate(p Policy, extraRoots ...string) *Gate {
	g := &Gate{P: p, Now: time.Now}
	for _, r := range append(slices.Clone(p.Roots), extraRoots...) {
		if r = Resolve(r); r != "" {
			g.roots = append(g.roots, r)
		}
	}
	for _, r := range p.ReadOnlyRoots {
		if r = Resolve(r); r != "" {
			g.ro = append(g.ro, r)
		}
	}
	return g
}

// Narrow returns a gate further limited to the client's MCP roots (file:// URIs already decoded).
func (g *Gate) Narrow(clientRoots []string) *Gate {
	n := *g
	n.client = nil
	for _, r := range clientRoots {
		n.client = append(n.client, Resolve(r))
	}
	return &n
}

// Level is the sandbox level processes run at.
func (g *Gate) Level() string { return g.P.SandboxLevel }

// Limits returns the policy limits.
func (g *Gate) Limits() Limits { return g.P.Limits }


func (g *Gate) grants(c string) []Grant {
	out := []Grant{}
	for _, gr := range g.P.Grants {
		if gr.Capability == c && gr.Live(g.Now()) {
			out = append(out, gr)
		}
	}
	return out
}

// Has reports whether a capability is on, by policy or by a live grant.
func (g *Gate) Has(c string) bool {
	if slices.Contains(g.P.Capabilities, c) || len(g.grants(c)) > 0 {
		return true
	}
	return c == CapRead && g.Has(CapWrite)
}

// Need fails with policy_denied when the capability is off.
func (g *Gate) Need(c string) error {
	if !g.Has(c) {
		return &Denied{"policy_denied", fmt.Sprintf("capability %s is off in this policy; a human can enable it with `devagotchi grant add --cap %s` (the model cannot)", c, c)}
	}
	return nil
}

// Path checks capability c (fs.read or fs.write) for p and returns the resolved path to use.
func (g *Gate) Path(c, p string) (string, error) {
	if err := g.Need(c); err != nil {
		return "", err
	}
	if strings.TrimSpace(p) == "" {
		return "", &Denied{"invalid", "path is empty"}
	}
	r := Resolve(p)
	if len(g.client) > 0 && !anyWithin(r, g.client) {
		return "", &Denied{"grant_required", fmt.Sprintf("%s is outside the client's MCP roots", r)}
	}
	if g.AnyPath {
		return r, nil
	}
	ok := c == CapRead && (anyWithin(r, g.ro) || anyWithin(r, g.roots))
	ok = ok || c == CapWrite && g.P.SandboxLevel != "read-only" && anyWithin(r, g.roots)
	for _, gr := range append(g.grants(CapWrite), g.grants(c)...) {
		for _, gp := range gr.Paths {
			ok = ok || Within(r, Resolve(gp))
		}
	}
	if !ok {
		return "", &Denied{"grant_required", fmt.Sprintf("%s is outside the %s roots of this policy (%s)", r, c, strings.Join(append(g.roots, g.ro...), ", "))}
	}
	return r, nil
}

// Host checks a URL host against url_hosts and net.fetch grants.
func (g *Gate) Host(h string) error {
	if err := g.Need(CapNet); err != nil {
		return err
	}
	hosts := slices.Clone(g.P.URLHosts)
	for _, gr := range g.grants(CapNet) {
		hosts = append(hosts, gr.Hosts...)
	}
	for _, pat := range hosts {
		if scope.HostOK(h, pat) {
			return nil
		}
	}
	return &Denied{"grant_required", fmt.Sprintf("host %s is not in url_hosts", h)}
}

func anyWithin(p string, roots []string) bool {
	for _, r := range roots {
		if Within(p, r) {
			return true
		}
	}
	return false
}
