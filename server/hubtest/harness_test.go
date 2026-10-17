// Package hubtest runs the real server (REST + MCP + SSE) against the agenthub_test database.
package hubtest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"agenthub/internal/auth"
	"agenthub/internal/config"
	"agenthub/internal/db"
	"agenthub/internal/httpapi"
	"agenthub/internal/mcptools"
	"agenthub/internal/presets"
	"agenthub/internal/store"
	"agenthub/internal/stream"

	"github.com/jackc/pgx/v5/pgxpool"
)

type env struct {
	t      *testing.T
	url    string
	st     *store.Store
	pool   *pgxpool.Pool
	tokens map[string]string
}

// setup wipes the test DB, migrates, seeds presets and starts an httptest server.
func setup(t *testing.T) *env {
	t.Helper()
	f := os.Getenv("HUB_TEST_DATABASE_URL_FILE")
	if f == "" {
		t.Skip("HUB_TEST_DATABASE_URL_FILE not set (run tests/run-go-tests.sh)")
	}
	b, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	url := strings.TrimSpace(string(b))
	ctx := context.Background()
	raw, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatal(err)
	}
	raw.Close()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(pool)
	p, _ := presets.Default()
	if _, err := st.Seed(ctx, store.SeedData{Settings: p.Settings, Roles: p.Roles, Principals: p.Principals,
		Workflows: p.Workflows, Projects: p.Projects, Policies: p.Policies}, "jesse@test"); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Listen: "127.0.0.1:0", HumanAuth: "none", PublicURL: "http://hub.test"}
	res, _ := auth.New(cfg, st)
	br := stream.New()
	bctx, cancel := context.WithCancel(ctx)
	go br.Run(bctx, pool)
	srv := &httpapi.Server{St: st, Auth: res, Broker: br, Cfg: cfg, MCP: mcptools.Handler(st, br, res)}
	hs := httptest.NewServer(srv.Handler())
	t.Cleanup(func() { hs.Close(); cancel(); pool.Close() })
	e := &env{t: t, url: hs.URL, st: st, pool: pool, tokens: map[string]string{}}
	for _, id := range []string{"jesse", "claude-code", "codex", "cursor", "clyffy"} {
		tok, err := st.CreateToken(ctx, store.System, id, "test", 0)
		if err != nil {
			t.Fatal(err)
		}
		e.tokens[id] = tok.Token
	}
	return e
}

type resp struct {
	Status int
	Body   map[string]any
	Header http.Header
}

func (r resp) code() string {
	if e, ok := r.Body["error"].(map[string]any); ok {
		return fmt.Sprint(e["code"])
	}
	return ""
}

// call does one REST request as principal who ("" = anonymous). Extra headers are "K: V" strings.
func (e *env) call(who, method, path string, body any, hdrs ...string) resp {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, e.url+"/api/v1"+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if who != "" {
		req.Header.Set("Authorization", "Bearer "+e.tokens[who])
	}
	for _, h := range hdrs {
		k, v, _ := strings.Cut(h, ": ")
		req.Header.Set(k, v)
	}
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer r.Body.Close()
	out := resp{Status: r.StatusCode, Header: r.Header, Body: map[string]any{}}
	raw, _ := io.ReadAll(r.Body)
	if json.Unmarshal(raw, &out.Body) != nil {
		var arr []any
		_ = json.Unmarshal(raw, &arr)
		out.Body = map[string]any{"list": arr}
	}
	return out
}
