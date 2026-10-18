#!/usr/bin/env bash
# One-time setup of the LIVE hub on this box: its own DB (agenthub_live), binaries, presets, tokens, and the
# box's own bridge enrolled as machine "clyffy-box". Idempotent. Prints no secrets (tokens go to 0600 files).
set -euo pipefail
. "$(dirname "$0")/live-env.sh"
HUB_PG_DBS=agenthub_live "$root/scripts/pg-up.sh" >/dev/null
[ -s "$HUB_SECRET_KEY_FILE" ] || (umask 077; head -c 32 /dev/urandom > "$HUB_SECRET_KEY_FILE")
"$(dirname "$0")/build.sh"
"$LIVE/bin/hub-server" admin seed --admin-login jesse >/dev/null
for p in jesse clyffy clyffy-infra-auditor; do
  f=$LIVE/secrets/$p.token
  [ -s "$f" ] || "$LIVE/bin/hub-server" admin token create --principal "$p" --out "$f" --name live >/dev/null
done
echo "bootstrap: db agenthub_live seeded; tokens in $LIVE/secrets (not printed)"
