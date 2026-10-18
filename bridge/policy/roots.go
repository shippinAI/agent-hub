package policy

import "slices"

// Roots lists the effective read+write and read-only roots: policy roots plus live fs grants,
// intersected with the client's MCP roots when it sent any. Processes are confined to these.
func (g *Gate) Roots() (rw, ro []string) {
	rw, ro = slices.Clone(g.roots), slices.Clone(g.ro)
	for _, gr := range g.grants(CapWrite) {
		rw = appendResolved(rw, gr.Paths)
	}
	for _, gr := range g.grants(CapRead) {
		ro = appendResolved(ro, gr.Paths)
	}
	if g.P.SandboxLevel == "read-only" {
		rw, ro = nil, append(ro, rw...)
	}
	if len(g.client) == 0 {
		return rw, ro
	}
	return g.intersect(rw), g.intersect(ro)
}

// Narrowed reports whether client MCP roots limit this gate.
func (g *Gate) Narrowed() []string { return g.client }

func (g *Gate) intersect(roots []string) []string {
	var out []string
	for _, r := range roots {
		for _, c := range g.client {
			switch {
			case Within(r, c):
				out = appendUnique(out, r)
			case Within(c, r):
				out = appendUnique(out, c)
			}
		}
	}
	return out
}

func appendResolved(dst, paths []string) []string {
	for _, p := range paths {
		if r := Resolve(p); r != "" {
			dst = appendUnique(dst, r)
		}
	}
	return dst
}

func appendUnique(dst []string, s string) []string {
	if slices.Contains(dst, s) {
		return dst
	}
	return append(dst, s)
}
