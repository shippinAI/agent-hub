package cli

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"slices"
	"time"

	"devagotchi/bridge/policy"
)

// cmdGrant manages local grants in the policy file: list | add | revoke ID. Humans only.
func cmdGrant(args []string) error {
	fs := flag.NewFlagSet("grant", flag.ContinueOnError)
	var pf policyFlags
	pf.bind(fs)
	var paths, hosts stringList
	capability := fs.String("cap", "", "add: capability (fs.read, fs.write, exec, proc.interactive, proc.list, proc.kill, net.fetch)")
	fs.Var(&paths, "path", "add: path the grant covers (repeatable; fs.* only)")
	fs.Var(&hosts, "host", "add: host pattern (repeatable; net.fetch only, e.g. *.github.com)")
	ttl := fs.Duration("ttl", 0, "add: lifetime such as 8h (0 = until revoked)")
	note := fs.String("note", "", "add: why")
	if len(args) == 0 {
		return fmt.Errorf("usage: devagotchi grant list | add --cap CAP [--path P]... [--host H]... [--ttl 8h] | revoke ID")
	}
	sub := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	p, _, err := pf.load()
	if err != nil {
		return err
	}
	now := time.Now()
	switch sub {
	case "list":
		for _, g := range p.Grants {
			state := "live"
			if !g.Live(now) {
				state = "expired"
			}
			exp := "until revoked"
			if !g.Expires.IsZero() {
				exp = g.Expires.Local().Format("2006-01-02 15:04 MST")
			}
			fmt.Printf("%s  %-16s %-7s %s paths=%v hosts=%v %s\n", g.ID, g.Capability, state, exp, g.Paths, g.Hosts, g.Note)
		}
		if len(p.Grants) == 0 {
			fmt.Println("No grants.")
		}
		return nil
	case "add":
		g := policy.Grant{ID: newID(), Capability: *capability, Hosts: hosts, Note: *note}
		for _, x := range paths {
			g.Paths = append(g.Paths, policy.Resolve(x))
		}
		if *ttl > 0 {
			g.Expires = now.Add(*ttl).UTC()
		}
		p.Grants = append(p.Grants, g)
		if err := policy.Save(pf.file(), p); err != nil {
			return err
		}
		fmt.Printf("Added grant %s (%s) to %s\n", g.ID, g.Capability, pf.file())
	case "revoke":
		n := len(p.Grants)
		p.Grants = slices.DeleteFunc(p.Grants, func(g policy.Grant) bool { return g.ID == fs.Arg(0) })
		if len(p.Grants) == n {
			return fmt.Errorf("no grant %q", fs.Arg(0))
		}
		if err := policy.Save(pf.file(), p); err != nil {
			return err
		}
		fmt.Printf("Revoked %s\n", fs.Arg(0))
	default:
		return fmt.Errorf("unknown grant command %q", sub)
	}
	return nil
}

func newID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return "g-" + hex.EncodeToString(b)
}
