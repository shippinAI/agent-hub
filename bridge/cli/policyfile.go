package cli

import (
	"flag"
	"os"
	"path/filepath"

	"devagotchi/bridge/config"
	"devagotchi/bridge/policy"
)

// stringList is a repeatable flag.
type stringList []string

func (s *stringList) String() string     { return "" }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

// policyFlags are shared by mcp, config, grant and doctor.
type policyFlags struct {
	dir, path string
}

func (f *policyFlags) bind(fs *flag.FlagSet) {
	fs.StringVar(&f.dir, "config-dir", config.UserDir(), "directory for policy.toml, history and http-token")
	fs.StringVar(&f.path, "policy", "", "policy file (default <config-dir>/policy.toml)")
}

func (f *policyFlags) file() string {
	if f.path != "" {
		return f.path
	}
	return filepath.Join(f.dir, policy.FileName)
}

// load reads the policy; a missing file means the shipped defaults (no roots: fail closed).
func (f *policyFlags) load() (policy.Policy, bool, error) {
	p, err := policy.Load(f.file())
	if os.IsNotExist(err) {
		return policy.Defaults(), false, nil
	}
	return p, err == nil, err
}
