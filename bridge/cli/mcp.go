package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"devagotchi/bridge/mcpserve"
)

// cmdMCP runs the standalone MCP server. stdout carries only the protocol in stdio mode; logs go to stderr.
func cmdMCP(args []string) error {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	var pf policyFlags
	pf.bind(fs)
	var roots stringList
	fs.Var(&roots, "root", "allowed root (repeatable; added to the policy roots)")
	useHTTP := fs.Bool("http", false, "serve streamable HTTP instead of stdio")
	addr := fs.String("addr", "", "HTTP listen address (default policy http_addr, 127.0.0.1:7676)")
	quiet := fs.Bool("quiet", false, "no log lines on stderr")
	if err := fs.Parse(args); err != nil {
		return err
	}
	p, found, err := pf.load()
	if err != nil {
		return fmt.Errorf("policy %s: %w", pf.file(), err)
	}
	logf := func(s string) {
		if !*quiet {
			fmt.Fprintln(os.Stderr, "devagotchi:", s)
		}
	}
	if !found {
		logf("no policy file at " + pf.file() + "; using defaults (`devagotchi config init` writes one)")
	}
	if len(p.Roots)+len(p.ReadOnlyRoots)+len(roots)+len(fs.Args()) == 0 {
		logf("no roots configured: file tools will refuse every path until you pass --root DIR or set roots in the policy")
	}
	self, _ := os.Executable()
	self, _ = filepath.EvalSymlinks(self)
	srv := mcpserve.New(mcpserve.Config{Policy: p, ConfigPath: pf.file(), ConfigDir: pf.dir,
		ExtraRoots: append(roots, fs.Args()...), Self: self, Log: logf})
	defer srv.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if !*useHTTP {
		return srv.RunStdio(ctx)
	}
	if *addr == "" {
		*addr = p.HTTPAddr
	}
	tok, err := mcpserve.Token(pf.dir)
	if err != nil {
		return err
	}
	return srv.RunHTTP(ctx, *addr, tok, func(a string) {
		logf(fmt.Sprintf("streamable HTTP on http://%s/mcp (bearer token in %s)", a, filepath.Join(pf.dir, mcpserve.TokenFile)))
	})
}
