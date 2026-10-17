// Package auth turns a request into a principal. Agents use per-principal bearer tokens; humans can
// also come in through a configured human mode. Identity never comes from request arguments.
package auth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strings"

	"agenthub/internal/config"
	"agenthub/internal/store"
)

// Resolver resolves principals.
type Resolver struct {
	st    *store.Store
	human func(*http.Request) (string, error)
	Mode  string
}

// New builds a resolver for the configured human mode (none | tailscale-whois | proxy-header | dev).
func New(c config.Config, st *store.Store) (*Resolver, error) {
	r := &Resolver{st: st, Mode: c.HumanAuth}
	switch c.HumanAuth {
	case "none", "token":
	case "tailscale-whois":
		r.human = func(req *http.Request) (string, error) { return tsWhois(c.TailscaleBin, req.RemoteAddr) }
	case "proxy-header":
		r.human = func(req *http.Request) (string, error) {
			if subtle.ConstantTimeCompare([]byte(req.Header.Get(c.HeaderSecret)), []byte(c.ProxySecret)) != 1 {
				return "", errors.New("missing proxy secret")
			}
			if u := strings.TrimSpace(req.Header.Get(c.HeaderUser)); u != "" {
				return u, nil
			}
			return "", errors.New("no identity header")
		}
	case "dev":
		host, _, _ := net.SplitHostPort(c.Listen)
		if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() || c.DevLogin == "" {
			return nil, errors.New("dev auth needs HUB_DEV_LOGIN and a loopback HUB_LISTEN")
		}
		r.human = func(*http.Request) (string, error) { return c.DevLogin, nil }
	default:
		return nil, fmt.Errorf("unknown HUB_HUMAN_AUTH %q", c.HumanAuth)
	}
	return r, nil
}

// Bearer extracts a bearer token, if any.
func Bearer(req *http.Request) string {
	h := req.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// Identify returns the principal and how it was authenticated ("token" or the human mode).
func (r *Resolver) Identify(req *http.Request) (*store.Principal, string, error) {
	if tok := Bearer(req); tok != "" {
		p, err := r.Token(req.Context(), tok)
		return p, "token", err
	}
	if r.human == nil {
		return nil, "", errors.New("send Authorization: Bearer <token>")
	}
	login, err := r.human(req)
	if err != nil {
		return nil, "", err
	}
	p, err := r.st.PrincipalByLogin(req.Context(), login)
	if err != nil {
		return nil, "", fmt.Errorf("login %q is not mapped to an enabled principal", login)
	}
	return p, r.Mode, nil
}

// Token resolves a bearer token.
func (r *Resolver) Token(ctx context.Context, tok string) (*store.Principal, error) {
	p, err := r.st.PrincipalByToken(ctx, tok)
	if err != nil {
		return nil, errors.New("invalid or revoked token")
	}
	return p, nil
}

// tsWhois asks tailscaled who owns the peer; tagged nodes have no user and are refused.
func tsWhois(bin, addr string) (string, error) {
	out, err := exec.Command(bin, "whois", "--json", addr).Output()
	if err != nil {
		return "", errors.New("not a tailnet peer")
	}
	var w struct {
		Node        struct{ Tags []string }
		UserProfile struct{ LoginName string }
	}
	if json.Unmarshal(out, &w) != nil || len(w.Node.Tags) > 0 || w.UserProfile.LoginName == "" {
		return "", errors.New("tagged or unknown tailnet node")
	}
	return w.UserProfile.LoginName, nil
}
