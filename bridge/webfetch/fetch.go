// Package webfetch is read_file's isUrl path: GET only, policy-checked host (and every redirect hop),
// no cookies, size and time caps, and private/loopback addresses refused unless the host is listed by name.
package webfetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Page is a fetched resource.
type Page struct {
	URL       string `json:"url"`
	Status    int    `json:"status"`
	Mime      string `json:"mime"`
	Body      []byte `json:"-"`
	Truncated bool   `json:"truncated,omitempty"`
}

// Rules decide per host. HostOK is the policy check; Named says the host is listed literally (not via "*").
type Rules struct {
	HostOK    func(host string) error
	Named     func(host string) bool
	Timeout   time.Duration
	MaxBytes  int64
	UserAgent string
}

// Fetch GETs u under the rules.
func Fetch(ctx context.Context, u string, r Rules) (Page, error) {
	pu, err := url.Parse(u)
	if err != nil || (pu.Scheme != "http" && pu.Scheme != "https") || pu.Hostname() == "" {
		return Page{}, fmt.Errorf("only http(s) URLs with a host can be read")
	}
	if err := r.HostOK(pu.Hostname()); err != nil {
		return Page{}, err
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	tr := &http.Transport{Proxy: nil, // no proxy hop: the private-address check must see the real target
		TLSHandshakeTimeout: 10 * time.Second,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, _ := net.SplitHostPort(addr)
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, ip := range ips {
				if private(ip.IP) && !r.Named(host) {
					return nil, fmt.Errorf("grant_required: %s resolves to a private address; list it by name in url_hosts to allow it", host)
				}
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
		}}
	c := &http.Client{Transport: tr, Timeout: r.Timeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		return r.HostOK(req.URL.Hostname())
	}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Page{}, err
	}
	req.Header.Set("User-Agent", r.UserAgent)
	resp, err := c.Do(req)
	if err != nil {
		return Page{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, r.MaxBytes+1))
	if err != nil {
		return Page{}, err
	}
	p := Page{URL: resp.Request.URL.String(), Status: resp.StatusCode, Mime: mime(resp.Header.Get("Content-Type"), body)}
	if int64(len(body)) > r.MaxBytes {
		body, p.Truncated = body[:r.MaxBytes], true
	}
	p.Body = body
	return p, nil
}

func mime(ct string, body []byte) string {
	if ct == "" {
		ct = http.DetectContentType(body)
	}
	m, _, _ := strings.Cut(ct, ";")
	return strings.TrimSpace(strings.ToLower(m))
}

func private(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}
