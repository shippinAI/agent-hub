package providers

import (
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var (
	idRe      = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)+$`)
	parserRe  = regexp.MustCompile(`^[a-z0-9_.-]+@[0-9]+(\.[0-9]+)*$`)
	refNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_.:-]{0,63}$`)
	iconRe    = regexp.MustCompile(`^[a-z0-9][a-z0-9/_.-]{0,127}$`)
	secretEnv = regexp.MustCompile(`(?i)(key|token|secret|password|credential)`)
	secretRef = regexp.MustCompile(`^\{\{secret:[a-z0-9][a-z0-9_.:-]{0,63}\}\}$`)
)

// Credential stores the bridge never reads (doc 32 rule 3). A detect dir or state path that names one is refused.
var credentialNames = []string{"auth.json", "credentials", "credentials.json", ".credentials.json", "token", "tokens",
	"keychain", "oauth_creds.json", "hosts.yml", "id_rsa", "id_ed25519"}

func text(errs *Errs, p, v string, required bool, max int) {
	switch {
	case required && strings.TrimSpace(v) == "":
		errs.add(p, "is required")
	case len(v) > max:
		errs.add(p, "is longer than %d characters", max)
	}
}

func enum(errs *Errs, p, v string, allowed []string) {
	if !slices.Contains(allowed, v) {
		errs.add(p, "must be one of %s, not %q", strings.Join(allowed, ", "), v)
	}
}

// set checks a non-empty (when required), duplicate-free subset of allowed.
func set(errs *Errs, p string, vs, allowed []string, required bool) {
	if required && len(vs) == 0 {
		errs.add(p, "needs at least one of %s", strings.Join(allowed, ", "))
	}
	seen := map[string]bool{}
	for i, v := range vs {
		if seen[v] {
			errs.add(p, "lists %q twice", v)
		}
		seen[v] = true
		if !slices.Contains(allowed, v) {
			errs.add(p+"["+itoa(i)+"]", "must be one of %s, not %q", strings.Join(allowed, ", "), v)
		}
	}
}

func link(errs *Errs, p, v string) {
	if v == "" {
		return
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		errs.add(p, "must be an http(s) URL")
	}
}

func argv(errs *Errs, p string, v []string, required bool) {
	if required && len(v) == 0 {
		errs.add(p, "needs at least the program name")
	}
	for i, a := range v {
		if a == "" || strings.ContainsRune(a, 0) {
			errs.add(p+"["+itoa(i)+"]", "must be a non-empty argument")
		}
	}
}

// env refuses literal secrets: a variable whose name looks secret must be a {{secret:name}} reference.
func env(errs *Errs, p string, m map[string]string) {
	for k, v := range m {
		if k == "" || strings.ContainsAny(k, "= \t") {
			errs.add(p, "has an invalid variable name %q", k)
		}
		if secretEnv.MatchString(k) && !secretRef.MatchString(v) {
			errs.add(join(p, k), "looks like a secret; use a {{secret:name}} reference, never the value")
		}
	}
}

// credentialPath reports whether a configured path points at a credential store.
func credentialPath(p string) bool {
	base := strings.ToLower(path.Base(strings.ReplaceAll(p, `\`, "/")))
	return slices.Contains(credentialNames, base) || strings.HasPrefix(base, "credentials")
}

func itoa(i int) string { return strconv.Itoa(i) }
