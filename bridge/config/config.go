// Package config is the bridge's local state: bridge.toml and the 0600 token file in the per-OS config dir.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/BurntSushi/toml"
)

// Config is bridge.toml.
type Config struct {
	HubURL         string   `toml:"hub_url"`
	MachineID      string   `toml:"machine_id"`
	ShellProfiles  []string `toml:"shell_profile_ids"`
	LogLevel       string   `toml:"log_level"`
	PollSeconds    int      `toml:"bridge_poll_seconds"`
	CacheSeconds   int      `toml:"grant_cache_seconds"`
	SecretPatterns []string `toml:"secret_patterns"`
	Transport      string   `toml:"transport"` // ws (default) or poll
}

// Dir is the per-OS config dir (overridable with DEVAGOTCHI_CONFIG_DIR, used by tests and the e2e run).
func Dir() string {
	if d := os.Getenv("DEVAGOTCHI_CONFIG_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Devagotchi")
	case "windows":
		if pd := os.Getenv("ProgramData"); pd != "" {
			return filepath.Join(pd, "Devagotchi")
		}
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "devagotchi")
	}
	return filepath.Join(home, ".config", "devagotchi")
}

// Load reads bridge.toml.
func Load(dir string) (Config, error) {
	var c Config
	if _, err := toml.DecodeFile(filepath.Join(dir, "bridge.toml"), &c); err != nil {
		return c, fmt.Errorf("read bridge.toml (run `devagotchi-bridge enroll` first): %w", err)
	}
	if c.CacheSeconds <= 0 || c.CacheSeconds > 30 {
		c.CacheSeconds = min(max(c.CacheSeconds, 1), 30)
	}
	if c.PollSeconds <= 0 {
		c.PollSeconds = 25
	}
	return c, nil
}

// Save writes bridge.toml (0600, dir 0700) via temp + rename.
func Save(dir string, c Config) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp := filepath.Join(dir, ".bridge.toml.tmp")
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err := toml.NewEncoder(f).Encode(c); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	f.Close()
	return os.Rename(tmp, filepath.Join(dir, "bridge.toml"))
}

// WriteToken stores the machine token owner-only. It is never printed.
func WriteToken(dir, tok string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	p := filepath.Join(dir, "token")
	_ = os.Remove(p)
	return os.WriteFile(p, []byte(tok), 0o600)
}

// ReadToken reads the token, refusing a group/world-readable file on Unix.
func ReadToken(dir string) (string, error) {
	p := filepath.Join(dir, "token")
	st, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	if runtime.GOOS != "windows" && st.Mode().Perm()&0o077 != 0 {
		return "", errors.New("token file is readable by others; chmod 600 it")
	}
	b, err := os.ReadFile(p)
	return string(b), err
}
