#!/usr/bin/env bash
# Starts a demo hub on 127.0.0.1:18080 (dev human auth = jesse@tailnet, loopback only) on the agenthub_ui DB
# and fills it through the real CLI as different agents. Touches: agenthub_ui DB, .run/ui/ (tokens 0600, logs).
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd); run=$root/.run/ui
"$root/scripts/pg-up.sh" >/dev/null
pkill -f "hub-server-ui" 2>/dev/null || true
rm -rf "$run" && mkdir -p "$run/tokens" && chmod 700 "$run" "$run/tokens"
url_file=$root/.run/secrets/db-url-agenthub_ui
bin=/usr/lib/postgresql/17/bin
"$bin/psql" "$(cat "$url_file")" -qc 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;' >/dev/null
(cd "$root/server" && go build -o bin/hub-server ./cmd/hub-server && go build -o bin/hub ./cmd/hub)
cp "$root/server/bin/hub-server" "$run/hub-server-ui"
export HUB_DATABASE_URL_FILE=$url_file
"$run/hub-server-ui" admin seed --admin-login jesse@tailnet >/dev/null
for a in jesse claude-code codex cursor clyffy; do
  "$run/hub-server-ui" admin token create --principal $a --name demo --out "$run/tokens/$a.token" >/dev/null
done
HUB_LISTEN=127.0.0.1:18080 HUB_PUBLIC_URL=http://hub.tailnet.example:8088 HUB_HUMAN_AUTH=dev HUB_DEV_LOGIN=jesse@tailnet \
  nohup "$run/hub-server-ui" > "$run/server.log" 2>&1 &
for i in $(seq 1 50); do curl -sf http://127.0.0.1:18080/healthz >/dev/null && break; sleep 0.1; done
export HUB_URL=http://127.0.0.1:18080
as() { local who=$1; shift; HUB_TOKEN_FILE=$run/tokens/$who.token HUB_STATE=$run/$who.claims.json "$root/server/bin/hub" "$@"; }
J() { curl -sf -X "$1" "$HUB_URL/api/v1$2" -H "Authorization: Bearer $(cat "$run/tokens/jesse.token")" -H 'Content-Type: application/json' -d "${3:-{\}}" >/dev/null; }
ver() { curl -sf "$HUB_URL/api/v1/config" -H "Authorization: Bearer $(cat "$run/tokens/jesse.token")" | jq ".settings[]|select(.key==\"$1\").version"; }
J PUT /settings/lease_min_ttl_seconds "{\"value\":2,\"version\":$(ver lease_min_ttl_seconds)}"
as jesse new "Postgres schema for agent-hub" --labels db,backend --priority 1 --desc "Tables, triggers for append-only events, readiness helpers." >/dev/null
as jesse new "MCP endpoint: streamable HTTP + bearer auth" --labels mcp,backend --priority 0 --desc "Stateless; identity from token; instructions <= 512 chars." >/dev/null
as jesse new "Board UI with lease countdowns" --labels ui --priority 1 >/dev/null
as jesse new "Write docs/connect.md for Claude, Codex, Cursor" --labels docs --priority 2 >/dev/null
as jesse new "Rotate Zitadel service-account keys" --labels security,human-only --priority 1 >/dev/null
as jesse new "Export events to the audit-plugin collector" --labels audit --priority 3 >/dev/null
as jesse new "Load test claim_next with 50 agents" --labels perf --priority 2 >/dev/null
J POST /tasks '{"project":"general","title":"Deploy agent-hub to the platform project","labels":["deploy"],"priority":2,"depends_on":[1,2]}'
as jesse new "Retry flaky reaper test" --labels tests --priority 2 >/dev/null
as jesse new "Plan the git JSONL export" --labels docs --priority 4 >/dev/null
# claude-code: works #2 and hands it off to codex
as claude-code claim 2 --ttl 900 >/dev/null
as claude-code note 2 "Using the official Go MCP SDK in stateless mode; JSON responses." --kind decision >/dev/null
as claude-code handoff 2 "Handler + auth done and tested with curl. Next: add the go-sdk client test and check the 512-char instructions limit." --to codex >/dev/null
# codex picks up the handoff (assignee first) and keeps a live lease
as codex claim --ttl 900 >/dev/null
as codex beat 2 >/dev/null
# cursor: schema task finished -> review; clyffy reviews it
as cursor claim 1 --ttl 900 >/dev/null
as cursor done 1 "Schema + triggers in 001_schema.sql; verified with go test ./hubtest." >/dev/null
as clyffy claim 1 --ttl 1200 >/dev/null
as clyffy note 1 "Checked the TRUNCATE trigger too; looks good." >/dev/null
# claude-code takes the UI task with a long lease; cursor takes the docs task with a short one that will expire
as claude-code claim 3 --ttl 1800 >/dev/null
as cursor claim 4 --ttl 2 >/dev/null
sleep 2.5
as codex note 7 "Blocked until the reaper interval is configurable per test." --kind blocker >/dev/null
as cursor claim 9 --ttl 600 >/dev/null
as cursor done 9 "Fixed by calling ReapOnce directly in tests." >/dev/null
echo "demo hub on http://127.0.0.1:18080 (tokens in $run/tokens, not printed)"
