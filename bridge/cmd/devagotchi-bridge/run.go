package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"devagotchi/bridge/config"
	"devagotchi/bridge/hubclient"
	"devagotchi/bridge/runner"
	"devagotchi/bridge/wire"
)

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	dir := fs.String("config-dir", config.Dir(), "config directory")
	once := fs.Bool("once", false, "process one poll batch and exit (for tests)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*dir)
	if err != nil {
		return err
	}
	tok, err := config.ReadToken(*dir)
	if err != nil {
		return fmt.Errorf("read token: %w", err)
	}
	self, _ := os.Executable()
	c := hubclient.New(cfg.HubURL, tok)
	r := runner.New(cfg, c, wire.SignKey(tok), self)
	r.ConfigDir = *dir
	logf := logger(*dir, cfg.LogLevel)
	r.Log = logf
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logf(fmt.Sprintf("bridge %s up for machine %s via %s", version, cfg.MachineID, cfg.Transport))
	if cfg.Transport == "ws" && !*once {
		return r.WSRun(ctx)
	}
	return pollRun(ctx, c, r, *once)
}

func pollRun(ctx context.Context, c *hubclient.Client, r *runner.Runner, once bool) error {
	for ctx.Err() == nil {
		frames, err := c.Poll(ctx)
		if err != nil {
			if once {
				return err
			}
			time.Sleep(time.Second)
			continue
		}
		for _, f := range frames {
			r.Handle(ctx, f)
		}
		if once {
			time.Sleep(300 * time.Millisecond)
			return nil
		}
	}
	return nil
}

func logger(dir, level string) func(string) {
	_ = level
	f, err := os.OpenFile(filepath.Join(dir, "bridge.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return func(s string) { log.Println(s) }
	}
	l := log.New(f, "", log.LstdFlags)
	return func(s string) { l.Println(s) }
}
