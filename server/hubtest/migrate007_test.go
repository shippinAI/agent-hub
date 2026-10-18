package hubtest

import (
	"context"
	"os"
	"strings"
	"testing"

	"agenthub/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Migration 007 converts old-shape provider rows in place and keeps every old value in providers_legacy_007.
func TestMigration007KeepsData(t *testing.T) {
	f := os.Getenv("HUB_TEST_DATABASE_URL_FILE")
	if f == "" {
		t.Skip("HUB_TEST_DATABASE_URL_FILE not set (run tests/run-go-tests.sh)")
	}
	b, _ := os.ReadFile(f)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatal(err)
	}
	if err := db.MigrateUntil(ctx, pool, "006_change_policy.sql"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO router_endpoints(id,label,url,kind) VALUES('litellm-local','l','http://127.0.0.1:4000','litellm');
		INSERT INTO providers(id,kind,label,models,router_endpoint_id,cli_name,terms_url,terms_status,terms_note) VALUES
		('anthropic-api','api','Anthropic API','["claude-sonnet","claude-opus"]','litellm-local',null,'https://t','verified','note a'),
		('claude-subscription','subscription_cli','Claude Pro','[]',null,'claude','','unverified','note b')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var kind, router, argv0, note string
	if err := pool.QueryRow(ctx, `SELECT adapter->>'kind', adapter->'config'->'router'->>'endpoint_id', terms_note FROM providers WHERE id='anthropic-api'`).Scan(&kind, &router, &note); err != nil {
		t.Fatal(err)
	}
	if kind != "anthropic_compatible" || router != "litellm-local" || note != "note a" {
		t.Fatalf("api row: %s %s %s", kind, router, note)
	}
	if err := pool.QueryRow(ctx, `SELECT adapter->>'kind' || ':' || (adapter->'config'->'argv'->>0) FROM providers WHERE id='claude-subscription'`).Scan(&argv0); err != nil || argv0 != "headless_json:claude" {
		t.Fatalf("cli row: %s %v", argv0, err)
	}
	var models, legacy int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM provider_models WHERE provider_id='anthropic-api'`).Scan(&models)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM providers_legacy_007 WHERE terms_status IS NOT NULL`).Scan(&legacy)
	if models != 2 || legacy != 2 {
		t.Fatalf("models %d legacy %d", models, legacy)
	}
	var col int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_name='providers' AND column_name IN ('terms_status','kind','cli_name')`).Scan(&col)
	if col != 0 {
		t.Fatal("old provider columns still present")
	}
}
