// hub-server: the agent-hub HTTP server (REST + SSE + MCP + UI) and its admin subcommands.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"agenthub/internal/auth"
	"agenthub/internal/config"
	"agenthub/internal/db"
	"agenthub/internal/devapi"
	"agenthub/internal/secretbox"
	"agenthub/internal/store"
	"agenthub/internal/stream"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	c, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	pool, err := db.Open(ctx, c.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	st := store.New(pool)
	if len(os.Args) > 1 && os.Args[1] == "admin" {
		if err := admin(ctx, st, os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}
	res, err := auth.New(c, st)
	if err != nil {
		log.Fatal(err)
	}
	br := stream.New()
	go br.Run(ctx, pool)
	go st.RunReaper(ctx)
	box, err := secretbox.Open(c.SecretKeyFile)
	if err != nil {
		log.Fatal(err)
	}
	srv, api := devapi.Build(ctx, st, res, br, c, box)
	go api.Access.RunReaper(ctx, 2*time.Second)
	hs := &http.Server{Addr: c.Listen, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		sh, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = hs.Shutdown(sh)
	}()
	log.Printf("agent-hub listening on %s (human auth: %s)", c.Listen, c.HumanAuth)
	if err := hs.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
