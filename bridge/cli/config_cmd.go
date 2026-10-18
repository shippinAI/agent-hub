package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"

	"devagotchi/bridge/policy"
)

// cmdConfig is the human-only policy editor. No MCP tool reaches this code.
func cmdConfig(args []string) error {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	var pf policyFlags
	pf.bind(fs)
	var roots, ro stringList
	fs.Var(&roots, "root", "init: read+write root (repeatable)")
	fs.Var(&ro, "read-only-root", "init: read-only root (repeatable)")
	force := fs.Bool("force", false, "init: overwrite an existing policy")
	if len(args) == 0 {
		return fmt.Errorf("usage: devagotchi config path | show | init [--root DIR]... | set KEY VALUE")
	}
	sub := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	switch sub {
	case "path":
		fmt.Println(pf.file())
	case "show":
		p, found, err := pf.load()
		if err != nil {
			return err
		}
		if !found {
			fmt.Fprintln(os.Stderr, "# no policy file yet; these are the defaults")
		}
		return toml.NewEncoder(os.Stdout).Encode(p)
	case "init":
		if _, err := os.Stat(pf.file()); err == nil && !*force {
			return fmt.Errorf("%s exists (use --force to replace it)", pf.file())
		}
		p := policy.Defaults()
		for _, r := range roots {
			p.Roots = append(p.Roots, policy.Resolve(r))
		}
		for _, r := range ro {
			p.ReadOnlyRoots = append(p.ReadOnlyRoots, policy.Resolve(r))
		}
		if err := policy.Save(pf.file(), p); err != nil {
			return err
		}
		fmt.Printf("Wrote %s (roots: %v). Edit it, or use `devagotchi config set` / `devagotchi grant add`.\n", pf.file(), p.Roots)
	case "set":
		if fs.NArg() != 2 {
			return fmt.Errorf("usage: devagotchi config set KEY VALUE (e.g. limits.max_timeout_ms 600000, roots '[\"~/code\"]')")
		}
		p, _, err := pf.load()
		if err != nil {
			return err
		}
		if p, err = policy.Set(p, fs.Arg(0), fs.Arg(1)); err != nil {
			return err
		}
		if err := policy.Save(pf.file(), p); err != nil {
			return err
		}
		fmt.Printf("Set %s in %s\n", fs.Arg(0), pf.file())
	default:
		return fmt.Errorf("unknown config command %q", sub)
	}
	return nil
}
