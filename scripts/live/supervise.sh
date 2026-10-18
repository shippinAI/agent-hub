#!/usr/bin/env bash
# Keeps the live stack running on this box: Postgres (pg_ctl), hub-server (REST + MCP + SSE + embedded web UI),
# and the box's own devagotchi-bridge (machine clyffy-box). Each child is restarted with backoff when it exits.
# Started detached by hubctl.sh; stops everything on TERM. Logs: .run/live/logs/*.log (no secrets).
set -uo pipefail
. "$(dirname "$0")/live-env.sh"
pgbin=${PG_BIN:-$(ls -d /usr/lib/postgresql/*/bin | sort -V | tail -1)}
kids=()
trap 'kill "${kids[@]}" 2>/dev/null; pkill -TERM -P $$ 2>/dev/null; exit 0' TERM INT

keep() { # keep NAME CMD... : run, restart on exit (1 s doubling to 30 s; reset after 60 s healthy)
  local name=$1; shift; local back=1
  while true; do
    local t0=$(date +%s)
    echo "$(date -Is) start $name" >> "$LIVE/logs/supervise.log"
    "$@" >> "$LIVE/logs/$name.log" 2>&1 &
    local pid=$!; echo "$pid" > "$LIVE/$name.pid"; wait "$pid"; local rc=$?
    echo "$(date -Is) $name exited rc=$rc" >> "$LIVE/logs/supervise.log"
    [ $(( $(date +%s) - t0 )) -gt 60 ] && back=1
    sleep "$back"; back=$(( back * 2 > 30 ? 30 : back * 2 ))
  done
}

pg_watch() {
  while true; do
    "$pgbin/pg_ctl" -D "$root/.run/pg" status >/dev/null 2>&1 || {
      echo "$(date -Is) starting postgres" >> "$LIVE/logs/supervise.log"
      "$pgbin/pg_ctl" -D "$root/.run/pg" -l "$root/.run/pg.log" -w start >/dev/null 2>&1; }
    sleep 10
  done
}

hub() {
  until "$pgbin/pg_ctl" -D "$root/.run/pg" status >/dev/null 2>&1; do sleep 1; done
  HUB_LISTEN=$LISTEN HUB_HUMAN_AUTH=$AUTH HUB_PUBLIC_URL=$URL exec "$LIVE/bin/hub-server"
}

bridge() {
  until curl -sf -o /dev/null "$URL/healthz"; do sleep 1; done
  [ -s "$LIVE/bridge/token" ] || { echo "bridge not enrolled yet (scripts/live/enroll-box.sh)"; sleep 30; return 1; }
  exec "$LIVE/bin/devagotchi-bridge" run --config-dir "$LIVE/bridge"
}

echo $$ > "$LIVE/supervise.pid"
pg_watch & kids+=($!)
keep hub-server hub & kids+=($!)
keep bridge bridge & kids+=($!)
wait
