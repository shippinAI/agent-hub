#!/usr/bin/env bash
# Full UI + API pass: builds the web app, starts the demo hub (tests/ui-demo.sh), drives it in headless Chrome
# (1280px + 390px), runs an independent MCP client (TypeScript SDK) and HTTP safety checks.
# Touches: agenthub_ui DB, .run/ui/, test-evidence/. Writes test-evidence/UI-SUMMARY.txt. Prints no secrets.
set -uo pipefail
root=$(cd "$(dirname "$0")/.." && pwd); ev=$root/test-evidence; url=http://127.0.0.1:18080; toks=$root/.run/ui/tokens
mkdir -p "$ev/screenshots" && rm -f "$ev"/screenshots/*.png
(cd "$root/web" && npm run build >/dev/null) || { echo "FAIL web build"; exit 1; }
"$root/tests/ui-demo.sh" >/dev/null || { echo "FAIL demo setup"; exit 1; }
(cd "$root/tests/ui" && node shots.mjs "$url" "$ev/screenshots" "$toks" "$ev/ui-checks.txt") >/dev/null 2>&1
(cd "$root/tests/mcp-node" && node client.mjs "$url" "$toks") > "$ev/mcp-node.txt" 2>&1
api=$ev/api-checks.txt; : > "$api"
ok() { if [ "$1" = "$2" ]; then echo "PASS api: $3" >> "$api"; else echo "FAIL api: $3 (got $1, want $2)" >> "$api"; fi; }
code() { curl -s -o /dev/null -w '%{http_code}' "$@"; }
ok "$(code -X POST "$url/api/v1/tasks" -H 'Content-Type: application/json' -d '{"title":"x"}')" 403 "browser-session POST without X-Hub-Client is refused (CSRF)"
ok "$(code -X POST "$url/api/v1/tasks" -H 'X-Hub-Client: ui' -H 'Content-Type: application/json' -d '{"project":"general","title":"csrf ok"}')" 200 "same request with X-Hub-Client is accepted"
ok "$(code -X POST "$url/mcp" -H 'Content-Type: application/json' -d '{}')" 401 "/mcp without a bearer token is 401"
ok "$(code "$url/api/v1/me" -H 'Authorization: Bearer ahk_not-a-real-token')" 401 "unknown bearer token is 401 (no dev fallback)"
hdr=$(curl -sI "$url/" | tr -d '\r')
grep -qi '^content-security-policy:' <<<"$hdr" && echo "PASS api: CSP header on UI" >> "$api" || echo "FAIL api: CSP header on UI" >> "$api"
out=$(HUB_DATABASE_URL_FILE=$root/.run/secrets/db-url-agenthub_ui HUB_LISTEN=0.0.0.0:18099 HUB_HUMAN_AUTH=dev HUB_DEV_LOGIN=x \
  timeout 5 "$root/.run/ui/hub-server-ui" 2>&1); rc=$?
[ $rc -ne 0 ] && [ $rc -ne 124 ] && grep -qi loopback <<<"$out" && echo "PASS api: dev auth refuses a non-loopback listen" >> "$api" \
  || echo "FAIL api: dev auth refuses a non-loopback listen (rc $rc)" >> "$api"
if grep -rlE 'ahk_[A-Za-z0-9_-]{20,}' "$ev" >/dev/null 2>&1; then echo "FAIL api: no full token in test-evidence" >> "$api"; else echo "PASS api: no full token in test-evidence" >> "$api"; fi
cat "$ev/ui-checks.txt" "$ev/mcp-node.txt" "$api" > "$ev/UI-SUMMARY.txt"
pass=$(grep -c '^PASS' "$ev/UI-SUMMARY.txt"); fail=$(grep -c '^FAIL' "$ev/UI-SUMMARY.txt")
echo "screenshots: $(ls "$ev/screenshots" | wc -l)" >> "$ev/UI-SUMMARY.txt"
echo "ui+api: $pass passed, $fail failed" | tee -a "$ev/UI-SUMMARY.txt"
[ "$fail" -eq 0 ]
