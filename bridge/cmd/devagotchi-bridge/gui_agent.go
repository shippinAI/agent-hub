//go:build linux

package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"devagotchi/bridge/config"
	"devagotchi/bridge/gui"
)

func cmdGUIAgent(args []string) error {
	fs := flag.NewFlagSet("gui-agent", flag.ContinueOnError)
	dir := fs.String("config-dir", config.Dir(), "config directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return gui.Serve(ctx, *dir, func(s string) { log.Println(s) })
}
