#!/usr/bin/env bash
# One real agent run, end to end, on a throwaway hub and a throwaway repo. Nothing is pushed and no PR is opened.
#
#   scripts/real-run.sh            # drives the real `claude` CLI (uses your Claude login or ANTHROPIC_API_KEY)
#   AGENT=fake scripts/real-run.sh # same wiring with a scripted agent, costs nothing
#
# It starts a private hub on 127.0.0.1:${REAL_RUN_PORT:-8097} with its own database (agenthub_realrun), seeds the
# default preset, files one small ticket ("add Reverse with a test") in a scratch Go repo, runs hub-seat --once as
# claude-code with `go test ./...` as the required check, then prints the task, its evidence, usage and run record.
# Env: REAL_RUN_ENV  names passed into the agent (default ANTHROPIC_API_KEY ANTHROPIC_BASE_URL)
#      SANDBOX       workspace-write or unsandboxed (default unsandboxed; HOME is the real one so a login works)
#      REAL_RUN_DB_URL_FILE  use this database (an empty one) instead of starting the box Postgres
#      KEEP=1        leave the hub running and the scratch dirs in place
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
port=${REAL_RUN_PORT:-8097}
work=$(mktemp -d "${TMPDIR:-/tmp}/real-run.XXXXXX")
bin=$work/bin; mkdir -p "$bin" "$work/secrets" && chmod 700 "$work/secrets"
url=http://127.0.0.1:$port

if [ -z "${REAL_RUN_DB_URL_FILE:-}" ]; then
  HUB_PG_DBS=agenthub_realrun "$root/scripts/pg-up.sh" >/dev/null
  REAL_RUN_DB_URL_FILE=$root/.run/secrets/db-url-agenthub_realrun
fi
(cd "$root/server" && go build -o "$bin/" ./cmd/hub-server ./cmd/hub-seat)
export HUB_DATABASE_URL_FILE=$REAL_RUN_DB_URL_FILE HUB_SECRET_KEY_FILE=$work/secrets/hub.key
export HUB_LISTEN=127.0.0.1:$port HUB_HUMAN_AUTH=none
(umask 077; head -c 32 /dev/urandom > "$HUB_SECRET_KEY_FILE")
"$bin/hub-server" admin seed >/dev/null
for p in jesse claude-code; do "$bin/hub-server" admin token create --principal "$p" --out "$work/secrets/$p" --name real-run >/dev/null; done
"$bin/hub-server" > "$work/hub.log" 2>&1 & hub=$!
export NO_PROXY=127.0.0.1,localhost no_proxy=127.0.0.1,localhost
cleanup() { kill "$hub" 2>/dev/null || true; [ "${KEEP:-}" = 1 ] || rm -rf "$work"; }
[ "${KEEP:-}" = 1 ] && echo "keeping $work (hub pid $hub)" || trap cleanup EXIT
for _ in $(seq 50); do curl --noproxy "*" -fs "$url/healthz" >/dev/null && break; sleep 0.2; done
curl --noproxy "*" -fs "$url/healthz" >/dev/null || { echo "hub did not start:"; tail -5 "$work/hub.log"; exit 1; }

api() { curl --noproxy '*' -fsS -H "Authorization: Bearer $(cat "$work/secrets/jesse")" -H 'Content-Type: application/json' "$@"; }

repo=$work/repo; mkdir -p "$repo"
git -C "$repo" init -q -b master
printf 'module scratch\n\ngo 1.22\n' > "$repo/go.mod"
printf 'package strutil\n\n// Upper is a placeholder so the package builds.\nfunc Upper(s string) string { return s }\n' > "$repo/strutil.go"
git -C "$repo" add . && git -C "$repo" -c user.name=t -c user.email=t@t commit -qm init

id=$(api -X POST "$url/api/v1/tasks" -d '{"title":"Add Reverse to strutil","project":"general",
  "description":"Add func Reverse(s string) string to strutil.go that reverses a string by runes (so it is safe for UTF-8), and a table test in strutil_test.go covering empty, ASCII and multi-byte input. Keep the change to those two files.",
  "acceptance":["Reverse reverses by runes","go test ./... passes"]}' | sed -n 's/.*"id":\([0-9]*\).*/\1/p' | head -1)
echo "ticket #$id filed"

agent=()
if [ "${AGENT:-}" = fake ]; then
  cat > "$bin/fake-agent" <<'EOF'
#!/bin/sh
cat >/dev/null
cat > strutil_test.go <<'T'
package strutil

import "testing"

func TestReverse(t *testing.T) {
	for in, want := range map[string]string{"": "", "abc": "cba", "héllo": "olléh"} {
		if got := Reverse(in); got != want {
			t.Errorf("Reverse(%q) = %q, want %q", in, got, want)
		}
	}
}
T
cat >> strutil.go <<'T'

// Reverse reverses s by runes.
func Reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}
T
echo '{"type":"system","subtype":"init","session_id":"fake-1","model":"fake"}'
echo '{"type":"result","subtype":"success","is_error":false,"result":"added Reverse","num_turns":1,"session_id":"fake-1","total_cost_usd":0,"modelUsage":{"fake":{"inputTokens":1,"outputTokens":1,"costUSD":0}}}'
EOF
  chmod +x "$bin/fake-agent"; agent=(--agent "$bin/fake-agent")
fi
envs=(); for k in ${REAL_RUN_ENV:-ANTHROPIC_API_KEY ANTHROPIC_BASE_URL}; do envs+=(--env "$k"); done

"$bin/hub-seat" --once --hub "$url" --token-file "$work/secrets/claude-code" --name real-run --repo "$repo" --base master \
  --work-root "$work/wt" --sandbox "${SANDBOX:-unsandboxed}" --timeout 15m --escalate-to jesse \
  --check 'test.unit=go test ./...' "${envs[@]}" "${agent[@]}"

det=$(api "$url/api/v1/tasks/$id")
echo "--- task #$id"; printf '%s' "$det" | jq '{state: .task.state, evidence: [.evidence[] | {kind, status, summary}], usage: .usage, runs: [.runs[] | {id, status, adapter, model, turns, session_id, transcript_sha256, reason}]}'
run=$(printf '%s' "$det" | jq -r '.runs[-1].id')
echo "--- diff on the task branch"; git -C "$repo" log -1 --stat --format='%an %s' "$(git -C "$repo" branch --list 'hub/*' --format='%(refname:short)' | tail -1)"
echo "--- chain"; api "$url/api/v1/events/verify" | jq -c .
echo "run record: $url/api/v1/runs/$run (hub stops on exit unless KEEP=1)"
