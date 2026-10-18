# Sourced by the live scripts. Box-only paths and ports; no secrets here (they live in $LIVE/secrets, 0600).
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
LIVE=${HUB_LIVE_DIR:-$root/.run/live}
PORT=${HUB_LIVE_PORT:-8088}
mkdir -p "$LIVE/bin" "$LIVE/logs" "$LIVE/secrets" "$LIVE/bridge" && chmod 700 "$LIVE/secrets" "$LIVE/bridge"
# Reachability: tailnet-only when tailscale is up on this box, else loopback only. Never 0.0.0.0.
TS_IP=$(command -v tailscale >/dev/null 2>&1 && tailscale ip -4 2>/dev/null | head -1 || true)
if [ -n "$TS_IP" ]; then
  LISTEN=$TS_IP:$PORT; AUTH=tailscale-whois
else
  LISTEN=127.0.0.1:$PORT; AUTH=none
fi
URL=http://$LISTEN
export HUB_DATABASE_URL_FILE=$root/.run/secrets/db-url-agenthub_live
export HUB_SECRET_KEY_FILE=$LIVE/secrets/hub-secret.key
