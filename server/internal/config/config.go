// Package config reads hub-server settings from the environment. Secrets come from *_FILE paths.
package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds deployment wiring only. Behaviour (leases, WIP, roles...) lives in the DB settings table.
type Config struct {
	Listen       string // HUB_LISTEN, default 127.0.0.1:8088
	DatabaseURL  string // from HUB_DATABASE_URL_FILE (preferred) or HUB_DATABASE_URL
	HumanAuth    string // HUB_HUMAN_AUTH: none | tailscale-whois | proxy-header | dev
	HeaderUser   string // HUB_PROXY_USER_HEADER (proxy-header)
	HeaderSecret string // HUB_PROXY_SECRET_HEADER (proxy-header)
	ProxySecret  string // from HUB_PROXY_SECRET_FILE
	TailscaleBin string // HUB_TAILSCALE_BIN
	DevLogin     string // HUB_DEV_LOGIN (dev, loopback only)
	PublicURL    string // HUB_PUBLIC_URL, used in generated connect snippets
}

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func readSecret(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("config: cannot read secret file %s: %w", path, err)
	}
	return strings.TrimSpace(string(b)), nil
}

// Load builds the config; it never logs secret values.
func Load() (Config, error) {
	c := Config{
		Listen:       env("HUB_LISTEN", "127.0.0.1:8088"),
		HumanAuth:    env("HUB_HUMAN_AUTH", "none"),
		HeaderUser:   env("HUB_PROXY_USER_HEADER", "X-Forwarded-Email"),
		HeaderSecret: env("HUB_PROXY_SECRET_HEADER", "X-Hub-Proxy-Secret"),
		TailscaleBin: env("HUB_TAILSCALE_BIN", "tailscale"),
		DevLogin:     env("HUB_DEV_LOGIN", ""),
	}
	c.PublicURL = env("HUB_PUBLIC_URL", "http://"+c.Listen)
	var err error
	if f := env("HUB_DATABASE_URL_FILE", ""); f != "" {
		if c.DatabaseURL, err = readSecret(f); err != nil {
			return c, err
		}
	} else {
		c.DatabaseURL = env("HUB_DATABASE_URL", "")
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("config: set HUB_DATABASE_URL_FILE (or HUB_DATABASE_URL)")
	}
	if c.HumanAuth == "proxy-header" {
		f := env("HUB_PROXY_SECRET_FILE", "")
		if f == "" {
			return c, fmt.Errorf("config: proxy-header needs HUB_PROXY_SECRET_FILE")
		}
		if c.ProxySecret, err = readSecret(f); err != nil {
			return c, err
		}
		if len(c.ProxySecret) < 16 {
			return c, fmt.Errorf("config: proxy secret must be >= 16 chars")
		}
	}
	return c, nil
}
