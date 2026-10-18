#!/usr/bin/env bash
# Build the live binaries (web UI is embedded in hub-server). Box-only.
set -euo pipefail
. "$(dirname "$0")/live-env.sh"
(cd "$root/web" && npm run build >/dev/null)
(cd "$root/server" && go build -o "$LIVE/bin/hub-server.new" ./cmd/hub-server && mv "$LIVE/bin/hub-server.new" "$LIVE/bin/hub-server")
(cd "$root/bridge" && go build -o "$LIVE/bin/devagotchi-bridge.new" ./cmd/devagotchi-bridge && mv "$LIVE/bin/devagotchi-bridge.new" "$LIVE/bin/devagotchi-bridge")
echo "built $LIVE/bin/hub-server and devagotchi-bridge"
