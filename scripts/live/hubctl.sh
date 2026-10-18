#!/usr/bin/env bash
# Control the live stack: hubctl.sh start | stop | restart | status. Box-only; serves on loopback, or on the
# tailnet IP when tailscale is up (never a public address). After a box restart, run `hubctl.sh start` again.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
. "$here/live-env.sh"
running() { [ -s "$LIVE/supervise.pid" ] && kill -0 "$(cat "$LIVE/supervise.pid")" 2>/dev/null; }
case "${1:-status}" in
  start)
    running && { echo "already running (pid $(cat "$LIVE/supervise.pid"))"; exit 0; }
    setsid nohup "$here/supervise.sh" >> "$LIVE/logs/supervise.log" 2>&1 < /dev/null &
    sleep 3; echo "started supervisor pid $(cat "$LIVE/supervise.pid" 2>/dev/null) -> $URL" ;;
  stop)
    running && kill -TERM -- "-$(cat "$LIVE/supervise.pid")" 2>/dev/null; sleep 1
    for f in hub-server bridge; do [ -s "$LIVE/$f.pid" ] && kill "$(cat "$LIVE/$f.pid")" 2>/dev/null; done
    rm -f "$LIVE"/*.pid; echo stopped ;;
  restart) "$0" stop; "$0" start ;;
  status)
    running && echo "supervisor: up (pid $(cat "$LIVE/supervise.pid"))" || echo "supervisor: down"
    for f in hub-server bridge; do
      p=$(cat "$LIVE/$f.pid" 2>/dev/null); kill -0 "$p" 2>/dev/null && echo "$f: up (pid $p)" || echo "$f: down"; done
    echo "url: $URL (auth: $AUTH)"
    code=$(curl -s -o /dev/null -w '%{http_code}' "$URL/api/v1/me"); echo "hub http: $code (401 = up, needs a token)"
    if [ -s "$LIVE/secrets/jesse.token" ]; then
      curl -s -H "Authorization: Bearer $(cat "$LIVE/secrets/jesse.token")" "$URL/api/v1/machines" | python3 -c '
import sys,json
for m in json.load(sys.stdin): print("machine %s: connected=%s online=%s last_ok=%s" % (m["id"], m["connected"], m["online"], m.get("last_exec_ok_at")))' 2>/dev/null
    fi ;;
  *) echo "usage: hubctl.sh start|stop|restart|status"; exit 2 ;;
esac
