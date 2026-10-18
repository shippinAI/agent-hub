package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"devagotchi/bridge/config"
	"devagotchi/bridge/hubclient"
	"devagotchi/bridge/sandbox"
)

func cmdEnroll(args []string) error {
	fs := flag.NewFlagSet("enroll", flag.ContinueOnError)
	hub := fs.String("hub", "", "hub base URL, e.g. https://hub.example.ts.net")
	dir := fs.String("config-dir", config.Dir(), "config directory")
	display := fs.String("display-server", detectDisplay(), "x11 | wayland | none")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *hub == "" {
		return fmt.Errorf("--hub is required")
	}
	fmt.Fprint(os.Stderr, "Enrollment code: ")
	sc := bufio.NewScanner(os.Stdin)
	if !sc.Scan() {
		return fmt.Errorf("no code entered")
	}
	code := strings.TrimSpace(sc.Text())
	if code == "" {
		return fmt.Errorf("empty code")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := hubclient.New(*hub, "")
	kind := sandbox.Kind()
	out, err := c.Enroll(ctx, code, runtime.GOOS, runtime.GOARCH, version, kind, *display)
	if err != nil {
		return err
	}
	if err := config.WriteToken(*dir, out.Token); err != nil {
		return err
	}
	cfg := config.Config{HubURL: *hub, MachineID: out.MachineID, LogLevel: "info", PollSeconds: out.PollSeconds,
		CacheSeconds: out.CacheSeconds, Transport: "ws"}
	if err := config.Save(*dir, cfg); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Enrolled as machine %q (sandbox: %s). Start the service with `devagotchi-bridge run`.\n", out.MachineID, kind)
	return nil
}

func detectDisplay() string {
	if runtime.GOOS != "linux" {
		return "native"
	}
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return "wayland"
	}
	if os.Getenv("DISPLAY") != "" {
		return "x11"
	}
	return "none"
}
