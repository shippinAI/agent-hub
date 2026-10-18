package policyrules

import (
	"net"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
)

var reCache sync.Map

// re compiles and caches a pattern; a bad pattern never matches (and is reported by Validate).
func re(p string) *regexp.Regexp {
	if v, ok := reCache.Load(p); ok {
		return v.(*regexp.Regexp)
	}
	r, err := regexp.Compile(p)
	if err != nil {
		r = regexp.MustCompile(`a\A`) // matches nothing
	}
	reCache.Store(p, r)
	return r
}

// Classify returns the sorted class names whose regexes match text.
func (d *Doc) Classify(text string) []string {
	var out []string
	for name, pats := range d.Classes {
		for _, p := range pats {
			if re(p).MatchString(text) {
				out = append(out, name)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

var hostRe = regexp.MustCompile(`(?:[A-Za-z0-9_.-]+@)?([A-Za-z0-9][A-Za-z0-9.-]*[A-Za-z0-9])`)
var hostCmd = regexp.MustCompile(`(?:^|[;&|\s(])(?:ssh|scp|rsync|sftp|curl|wget|ping|nc|telnet|mosh|ansible|incus\s+remote\s+add)\s+([^;&|\n]*)`)

// Hosts pulls likely target hosts out of a command: the arguments of ssh/scp/curl/ping-like tools.
func Hosts(text string) []string {
	var out []string
	for _, m := range hostCmd.FindAllStringSubmatch(text, -1) {
		for _, tok := range strings.Fields(m[1]) {
			if strings.HasPrefix(tok, "-") {
				continue
			}
			tok = strings.TrimPrefix(strings.TrimPrefix(tok, "https://"), "http://")
			tok, _, _ = strings.Cut(tok, "/")
			if h := hostRe.FindStringSubmatch(tok); h != nil && (strings.Contains(h[1], ".") || strings.Contains(tok, "@") || strings.Contains(h[1], "-")) {
				out = append(out, strings.ToLower(strings.Split(h[1], ":")[0]))
			}
		}
	}
	return out
}

// RegionsFor maps hosts and site codes to region codes using the region table.
func (d *Doc) RegionsFor(hosts, sites []string) []string {
	set := map[string]bool{}
	for _, s := range sites {
		for _, r := range d.Regions {
			if strings.EqualFold(strings.TrimSpace(s), r.Code) || strings.EqualFold(strings.TrimSpace(s), r.Name) {
				set[r.Code] = true
			}
		}
	}
	for _, h := range hosts {
		for _, r := range d.Regions {
			for _, g := range r.Hosts {
				if hostMatch(g, h) {
					set[r.Code] = true
				}
			}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func hostMatch(glob, host string) bool {
	if _, n, err := net.ParseCIDR(glob); err == nil {
		ip := net.ParseIP(host)
		return ip != nil && n.Contains(ip)
	}
	ok, _ := path.Match(strings.ToLower(glob), strings.ToLower(host))
	return ok
}
