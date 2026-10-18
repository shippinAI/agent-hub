# 15. Live on the box: services, reachability, bundles

## What runs
`scripts/live/hubctl.sh start|stop|restart|status` starts `supervise.sh` detached (setsid). It keeps three
children alive, restarting each with backoff (1 s doubling to 30 s):
- Postgres (`.run/pg`; database `agenthub_live`, separate from the dev and test DBs)
- `hub-server`: REST, MCP, SSE and the embedded web UI on port 8088
- the box's own `devagotchi-bridge`, enrolled as machine `clyffy-box` (sandbox: bubblewrap)

Logs: `.run/live/logs/*.log`. Secrets (0600, never printed): `.run/live/secrets/`.
First time only: `scripts/live/bootstrap.sh` creates the DB, seeds presets and writes tokens, then
`scripts/live/enroll-box.sh` pipes a fresh enrollment code straight into `devagotchi-bridge enroll` on stdin.

This box has no systemd user bus (pid 1 is tini) and no cron daemon, so after a box restart run
`hubctl.sh start`. On hosts that do have a user bus, `deploy/systemd-user/agenthub-live.service` does the same job.

## Reachability
`live-env.sh` binds to the tailnet IP and uses `tailscale-whois` auth when `tailscale status` works.
Otherwise it binds to 127.0.0.1 with bearer-token auth. It never binds 0.0.0.0. Tailscale is not installed
on the box today, so the hub is loopback-only. To serve tailnet-only: install tailscale, `tailscale up`
(Jesse's approval and login), then `hubctl.sh restart`.

## Status
"Online" in Machines means the bridge is connected **and** a real call succeeded recently. "connected"
means the socket is up but no call has been proven yet.

## Proof
- `node tests/mcp-node/review.mjs <url> <agent> <reviewer> <human> clyffy-box`: the live reviewer flow over
  MCP, using real calls on the bridge.
- `node tests/ui/gate-shots.mjs <url> <human.token> <ticket>`: Playwright shots at 1280 and 390, saved to
  `test-evidence/screenshots/gate-*.png`.

## gold-policy hub mode
Set `GOLD_POLICY_HUB_URL` and `GOLD_POLICY_HUB_TOKEN_FILE` and gold-policy's hook, check and wrap ask
`POST /api/v1/policy/decide`. If the hub can't be reached, the call is denied with exit 2. See the gold-policy README.

## Install bundles (built, NOT installed)
`scripts/make-bundles.sh [hub-url]` writes `dist/bundles/<host>/` plus a `.tar.gz` (Linux) or `.zip`
(Windows). Hosts are data in `deploy/bundles/hosts/*.env`: clyffy-01 (Linux, arch unknown, so it ships both
amd64 and arm64) and gus, gippy and dabo (Windows, amd64 and arm64). One command per host (`./install.sh`,
or `install.ps1` in an elevated shell). The enrollment code comes from hub > Machines > Add machine and is
typed at a hidden prompt. Every install needs Jesse's approval and a tailnet hub URL.
