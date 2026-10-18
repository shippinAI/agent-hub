#!/usr/bin/env bash
# Devagotchi UI pass 2: the guide editor/runner, guide updates inbox, GUI-session live view, role-bindings
# matrix and Catalog settings tab. Builds the web app, starts a box-only dev hub (loopback dev auth),
# seeds guides/subscriptions/bindings plus a pending guide update and a live GUI session, then screenshots
# every screen at 1280 and 390. Box-only; prints no secrets. Writes test-evidence/screenshots/devagotchi-*.
set -uo pipefail
root=$(cd "$(dirname "$0")/.." && pwd); ev=$root/test-evidence; port=18079; url=http://127.0.0.1:$port
mkdir -p "$ev/screenshots"
"$root/scripts/pg-up.sh" >/dev/null
db=$(cat "$root/.run/secrets/db-url-agenthub_ui2" 2>/dev/null || cat "$root/.run/secrets/db-url-agenthub_ui")
key=$root/.run/secrets/hub-secret.key; [ -s "$key" ] || { head -c 32 /dev/urandom > "$key"; chmod 600 "$key"; }
(cd "$root/web" && npm run build >/dev/null) || { echo "FAIL web build"; exit 1; }
bin=$root/.run/ui/hub-dev2; mkdir -p "$root/.run/ui"
(cd "$root/server" && go build -o "$bin" ./cmd/hub-server) || { echo "FAIL server build"; exit 1; }
psql "$db" -v ON_ERROR_STOP=1 -qc 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;' >/dev/null 2>&1 || true
HUB_DATABASE_URL="$db" HUB_SECRET_KEY_FILE="$key" "$bin" admin seed --admin-login jesse@test >/dev/null
HUB_DATABASE_URL="$db" HUB_SECRET_KEY_FILE="$key" HUB_HUMAN_AUTH=dev HUB_DEV_LOGIN=jesse@test HUB_LISTEN=127.0.0.1:$port "$bin" >/tmp/devui2.log 2>&1 &
hub=$!; trap 'kill $hub 2>/dev/null' EXIT
for i in $(seq 1 40); do curl -sf "$url/api/v1/me" >/dev/null && break; sleep 0.25; done
h=(-H 'X-Hub-Client: ui' -H 'Content-Type: application/json')
# machine, subscriptions, role bindings
curl -s "${h[@]}" -X POST "$url/api/v1/machines" -d '{"id":"laptop","label":"Jesse Laptop","os":"linux","display_server":"x11"}' >/dev/null
curl -s "${h[@]}" -X POST "$url/api/v1/subscriptions" -d '{"label":"Anthropic (main)","provider_id":"anthropic-api","kind":"api_key"}' >/dev/null
curl -s "${h[@]}" -X POST "$url/api/v1/subscriptions" -d '{"label":"Claude on laptop","provider_id":"anthropic-claude-code","kind":"cli_login","machine_id":"laptop","worker_principal_id":"claude-code"}' >/dev/null
subs=$(curl -s "$url/api/v1/subscriptions")
s1=$(echo "$subs" | python3 -c "import sys,json;print(json.load(sys.stdin)[0]['id'])")
s2=$(echo "$subs" | python3 -c "import sys,json;print(json.load(sys.stdin)[1]['id'])")
curl -s "${h[@]}" -X POST "$url/api/v1/role-bindings" -d "{\"role_id\":\"worker\",\"chat_app\":\"claude\",\"subscription_id\":\"$s2\",\"priority\":0,\"fallback_subscription_id\":\"$s1\",\"enabled\":true}" >/dev/null
curl -s "${h[@]}" -X POST "$url/api/v1/role-bindings" -d "{\"role_id\":\"worker\",\"chat_app\":\"codex\",\"subscription_id\":\"$s1\",\"priority\":0,\"enabled\":true}" >/dev/null
curl -s "${h[@]}" -X POST "$url/api/v1/role-bindings" -d "{\"role_id\":\"builder\",\"chat_app\":\"claude\",\"subscription_id\":\"$s1\",\"priority\":0,\"enabled\":true}" >/dev/null
# a draft version (creates a local override of a shippin guide)
g=$(curl -s "$url/api/v1/guides/enroll-machine")
python3 - "$g" > /tmp/draft.json <<'PY'
import sys,json
g=json.loads(sys.argv[1]); 
print(json.dumps({"title":g['title'],"goal":g['goal'],"tags":g['tags'],"applies_to":g['applies_to'],
 "semver":"1.1.0","change_notes":"Clarify the test-call step and add a verify step.","risk_ids":g.get('risk_ids',[]),"steps":g.get('steps') or []}))
PY
curl -s "${h[@]}" -X POST "$url/api/v1/guides/enroll-machine/versions" -d @/tmp/draft.json >/dev/null
# a pending guide update and a live GUI session with one artifact
psql "$db" -v ON_ERROR_STOP=1 >/dev/null <<SQL
INSERT INTO guide_updates(guide_id,from_semver,to_semver,bundle_sha256,change_notes,payload,status)
VALUES('enroll-machine','1.0.0','1.1.0','sha'||md5('u'),
 'Shippin refined the enrolment flow: the bridge now verifies the pairing token before the first test call, and a new step checks the clock skew.',
 '{"steps":[]}'::jsonb,'pending');
WITH s AS (
  INSERT INTO gui_sessions(machine_id,grant_ids,holder,display_server,monitors,fence,ttl_seconds,lease_expires_at,state,started_at)
  VALUES('laptop','{}','claude-code','x11','[{"w":1920,"h":1080}]'::jsonb,1,900, now()+interval '12 minutes','active', now()-interval '3 minutes') RETURNING id)
INSERT INTO gui_artifacts(gui_session_id,kind,sha256,width,height,bytes,created_at)
  SELECT id,'screenshot', md5('f')||md5('f'), 1920,1080, 248193, now()-interval '8 seconds' FROM s;
UPDATE gui_sessions g SET last_artifact_id=(SELECT id FROM gui_artifacts a WHERE a.gui_session_id=g.id ORDER BY created_at DESC LIMIT 1) WHERE state='active';
SQL
(cd "$root/tests/ui" && npm install >/dev/null 2>&1; node devagotchi-shots2.mjs "$url" "$ev/screenshots" "$ev/devagotchi-ui2.txt")
pass=$(grep -c '^PASS' "$ev/devagotchi-ui2.txt"); fail=$(grep -c '^FAIL' "$ev/devagotchi-ui2.txt")
echo "devagotchi-ui2: $pass passed, $fail failed; screenshots: $(ls "$ev"/screenshots/devagotchi-*.png | wc -l)"
[ "$fail" -eq 0 ]
