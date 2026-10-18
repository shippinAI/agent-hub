#!/usr/bin/env bash
# Enroll this box's bridge as machine "clyffy-box" (needs the hub running). The code goes hub -> stdin of
# `devagotchi-bridge enroll` through a pipe; it is never printed or put on a command line.
set -euo pipefail
. "$(dirname "$0")/live-env.sh"
id=${1:-clyffy-box}
[ -s "$LIVE/bridge/token" ] && { echo "already enrolled ($LIVE/bridge)"; exit 0; }
J=$LIVE/secrets/jesse.token
h=(-s -H "X-Hub-Client: ui" -H "Content-Type: application/json" -H "Authorization: Bearer $(cat "$J")")
r=$(curl "${h[@]}" -X POST "$URL/api/v1/machines" -d "{\"id\":\"$id\",\"label\":\"Clyffy box (Linux)\",\"os\":\"linux\"}")
if ! echo "$r" | python3 -c "import sys,json; json.load(sys.stdin)['enrollment']['code']" >/dev/null 2>&1; then
  r=$(curl "${h[@]}" -X POST "$URL/api/v1/machines/$id/enroll-code" -d '{}')
fi
echo "$r" | python3 -c 'import sys,json; d=json.load(sys.stdin); print((d.get("enrollment") or d)["code"])' \
  | "$LIVE/bin/devagotchi-bridge" enroll --hub "$URL" --config-dir "$LIVE/bridge" --display-server none
