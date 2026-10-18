package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"agenthub/internal/presets"
	"agenthub/internal/store"
)

// admin runs bootstrap tasks as the "system" actor. Secrets go to files (0600), never to stdout.
func admin(ctx context.Context, st *store.Store, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: hub-server admin seed [--admin-login L] | token create --principal ID --out FILE [--name N] [--expires-days D] | audit anchor [--dir DIR]")
	}
	switch args[0] {
	case "seed":
		fs := flag.NewFlagSet("seed", flag.ContinueOnError)
		login := fs.String("admin-login", "", "external login (tailnet/proxy identity) for the preset human admin")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		n, err := presets.SeedAll(ctx, st, *login)
		if err != nil {
			return err
		}
		fmt.Printf("seeded (new rows): %v\n", n)
		return nil
	case "token":
		fs := flag.NewFlagSet("token", flag.ContinueOnError)
		pid := fs.String("principal", "", "principal id")
		out := fs.String("out", "", "file to write the token to (created 0600)")
		name := fs.String("name", "bootstrap", "token name")
		days := fs.Int("expires-days", 0, "0 = no expiry")
		if len(args) < 2 || args[1] != "create" {
			return errors.New("usage: token create --principal ID --out FILE")
		}
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if *pid == "" || *out == "" {
			return errors.New("--principal and --out are required (tokens are never printed)")
		}
		f, err := os.OpenFile(*out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return fmt.Errorf("open %s: %w (refusing to overwrite)", *out, err)
		}
		defer f.Close()
		tok, err := st.CreateToken(ctx, store.System, *pid, *name, *days)
		if err != nil {
			os.Remove(*out)
			return err
		}
		if _, err := fmt.Fprintln(f, tok.Token); err != nil {
			return err
		}
		fmt.Printf("token %s (%s…) for %s written to %s\n", tok.Info.ID, tok.Info.Prefix, *pid, *out)
		return nil
	case "audit":
		if len(args) < 2 || args[1] != "anchor" {
			return errAuditUsage
		}
		return auditAnchor(ctx, st, args[2:])
	}
	return fmt.Errorf("unknown admin command %q", args[0])
}
