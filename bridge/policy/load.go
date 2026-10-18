package policy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// FileName is the policy file inside the config dir.
const FileName = "policy.toml"

// Load reads a policy file and fills missing limits. A missing file is an error the caller can test with os.IsNotExist.
func Load(path string) (Policy, error) {
	p := Defaults()
	p.Grants = nil
	if _, err := toml.DecodeFile(path, &p); err != nil {
		return p, err
	}
	p.Limits.fill()
	return p, p.Validate()
}

// Validate rejects values the gate cannot enforce, so a broken file fails closed instead of open (DC #419).
func (p Policy) Validate() error {
	if !slices.Contains([]string{"read-only", "workspace-write", "unsandboxed"}, p.SandboxLevel) {
		return fmt.Errorf("sandbox_level %q must be read-only, workspace-write or unsandboxed", p.SandboxLevel)
	}
	known := []string{CapRead, CapWrite, CapExec, CapProc, CapProcList, CapProcKill, CapNet}
	for _, c := range p.Capabilities {
		if !slices.Contains(known, c) {
			return fmt.Errorf("unknown capability %q (known: %s)", c, strings.Join(known, ", "))
		}
	}
	for _, g := range p.Grants {
		if !slices.Contains(known, g.Capability) {
			return fmt.Errorf("grant %s: unknown capability %q", g.ID, g.Capability)
		}
	}
	if _, ok := p.Profile(p.Shell); !ok && p.Shell != "" {
		return fmt.Errorf("shell %q has no shell_profiles entry", p.Shell)
	}
	return nil
}

// Profile finds a shell profile by name.
func (p Policy) Profile(name string) (ShellProfile, bool) {
	for _, s := range p.ShellProfiles {
		if s.Name == name {
			return s, true
		}
	}
	return ShellProfile{}, false
}

// Save writes the policy owner-only via temp file + rename.
func Save(path string, p Policy) error {
	if err := p.Validate(); err != nil {
		return err
	}
	var b bytes.Buffer
	b.WriteString("# Devagotchi policy. Humans edit this (devagotchi config / grant, or an editor). No MCP tool can change it.\n")
	if err := toml.NewEncoder(&b).Encode(p); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b.Bytes(), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Set changes one key (dotted for limits, e.g. limits.max_timeout_ms). Lists take JSON or comma lists.
func Set(p Policy, key, value string) (Policy, error) {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(p); err != nil {
		return p, err
	}
	m := map[string]any{}
	if _, err := toml.Decode(buf.String(), &m); err != nil {
		return p, err
	}
	target, leaf := m, key
	if head, rest, ok := strings.Cut(key, "."); ok {
		sub, _ := m[head].(map[string]any)
		if sub == nil {
			return p, fmt.Errorf("unknown section %q", head)
		}
		target, leaf = sub, rest
	}
	old, ok := target[leaf]
	if !ok && !slices.Contains(listKeys, leaf) {
		return p, fmt.Errorf("unknown key %q", key)
	}
	nv, err := parseLike(old, leaf, value)
	if err != nil {
		return p, err
	}
	target[leaf] = nv
	buf.Reset()
	if err := toml.NewEncoder(&buf).Encode(m); err != nil {
		return p, err
	}
	out := Policy{}
	if _, err := toml.Decode(buf.String(), &out); err != nil {
		return p, err
	}
	out.Limits.fill()
	return out, out.Validate()
}

var listKeys = []string{"roots", "read_only_roots", "capabilities", "url_hosts", "egress_hosts", "secret_patterns", "prompt_patterns"}

func parseLike(old any, leaf, v string) (any, error) {
	switch old.(type) {
	case int64:
		return strconv.ParseInt(v, 10, 64)
	case bool:
		return strconv.ParseBool(v)
	case string:
		return v, nil
	}
	if slices.Contains(listKeys, leaf) {
		var l []string
		if strings.HasPrefix(strings.TrimSpace(v), "[") {
			err := json.Unmarshal([]byte(v), &l)
			return l, err
		}
		for _, s := range strings.Split(v, ",") {
			if s = strings.TrimSpace(s); s != "" {
				l = append(l, s)
			}
		}
		return l, nil
	}
	return nil, fmt.Errorf("key %q is not settable from the CLI; edit %s", leaf, FileName)
}
