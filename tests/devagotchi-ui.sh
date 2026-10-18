#!/usr/bin/env bash
# Devagotchi UI pass: build the web app, start a box-only dev hub (loopback dev auth), seed a machine,
# a pending access request, a live grant and a subscription, then screenshot every screen at 1280 and 390
# in headless Chrome. Box-only; prints no secrets. Writes test-evidence/screenshots/devagotchi-* and
# test-evidence/devagotchi-ui.txt.
set -uo pipefail
root=$(cd "$(dirname "$0")/.." && pwd); ev=$root/test-evidence; port=18077; url=http://127.0.0.1:$port
mkdir -p "$ev/screenshots"
"$root/scripts/pg-up.sh" >/dev/null
db=$(cat "$root/.run/secrets/db-url-agenthub_ui")
key=$root/.run/secrets/hub-secret.key; [ -s "$key" ] || { head -c 32 /dev/urandom > "$key"; chmod 600 "$key"; }
(cd "$root/web" && npm run build >/dev/null) || { echo "FAIL web build"; exit 1; }
bin=$root/.run/ui/hub-dev; mkdir -p "$root/.run/ui"
(cd "$root/server" && go build -o "$bin" ./cmd/hub-server) || { echo "FAIL server build"; exit 1; }
# fresh schema + seed
psql "$db" -v ON_ERROR_STOP=1 -qc 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;' >/dev/null 2>&1 || true
HUB_DATABASE_URL="$db" HUB_SECRET_KEY_FILE="$key" "$bin" admin seed --admin-login jesse@test >/dev/null
tok=$root/.run/ui/cc.token; HUB_DATABASE_URL="$db" HUB_SECRET_KEY_FILE="$key" "$bin" admin token create --principal claude-code --out "$tok" --name ui >/dev/null
HUB_DATABASE_URL="$db" HUB_SECRET_KEY_FILE="$key" HUB_HUMAN_AUTH=dev HUB_DEV_LOGIN=jesse@test HUB_LISTEN=127.0.0.1:$port "$bin" >/tmp/devui.log 2>&1 &
hub=$!; trap 'kill $hub 2>/dev/null' EXIT
for i in $(seq 1 40); do curl -sf "$url/api/v1/me" >/dev/null && break; sleep 0.25; done
CC=$(cat "$tok"); h=(-H 'X-Hub-Client: ui' -H 'Content-Type: application/json')
curl -s "${h[@]}" -X POST "$url/api/v1/machines" -d '{"id":"laptop","label":"Jesse Laptop","os":"linux"}' >/dev/null
R=$(curl -s -H "Authorization: Bearer $CC" -H 'Content-Type: application/json' -X POST "$url/api/v1/access-requests" -d '{"machine":"laptop","capability":"exec","selector":{"argv_prefix":[["git"],["npm"]],"paths":["/home/jesse/proj"]},"sandbox_level":"workspace-write","duration":"8h","reason":"Run git and npm in the project to help with the build."}')
id=$(echo "$R" | python3 -c "import sys,json;print(json.load(sys.stdin)['request_ids'][0])")
curl -s "${h[@]}" -X POST "$url/api/v1/access-requests/$id/approve" -d '{}' >/dev/null
curl -s -H "Authorization: Bearer $CC" -H 'Content-Type: application/json' -X POST "$url/api/v1/access-requests" -d '{"machine":"laptop","capability":"fs.read","selector":{"paths":["/home/jesse/logs"]},"sandbox_level":"read-only","duration":"1h","reason":"Read the log files to diagnose the failing test run."}' >/dev/null
curl -s "${h[@]}" -X POST "$url/api/v1/subscriptions" -d '{"id":"anthropic-main","label":"Anthropic (main)","provider_id":"anthropic-api","kind":"api_key"}' >/dev/null
(cd "$root/tests/ui" && npm install >/dev/null 2>&1; node devagotchi-shots.mjs "$url" "$ev/screenshots" "$ev/devagotchi-ui.txt")
pass=$(grep -c '^PASS' "$ev/devagotchi-ui.txt"); fail=$(grep -c '^FAIL' "$ev/devagotchi-ui.txt")
echo "devagotchi-ui: $pass passed, $fail failed; screenshots: $(ls "$ev"/screenshots/devagotchi-*.png | wc -l)"
[ "$fail" -eq 0 ]
