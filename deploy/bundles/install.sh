#!/usr/bin/env bash
# One-command Devagotchi bridge install for a Linux host:   ./install.sh   (run as the user the bridge acts as)
# 1. picks the binary for `uname -m`  2. asks for the enrollment code (hidden; never a flag, never echoed)
# 3. enrolls with the hub in host.env  4. installs + enables the systemd user unit (or prints the fallback).
# Needs Jesse's approval per host before it is run. It changes nothing outside ~/.local/bin and the bridge config dir.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
. "$here/host.env"
[ -n "${HUB_URL:-}" ] || { echo "host.env: HUB_URL is empty (set it to the hub's tailnet URL)"; exit 1; }
case "$HUB_URL" in *127.0.0.1*|*localhost*) echo "HUB_URL is loopback; a remote host needs the tailnet URL"; exit 1;; esac
case "$(uname -m)" in x86_64|amd64) arch=amd64;; aarch64|arm64) arch=arm64;; *) echo "unsupported arch $(uname -m)"; exit 1;; esac
bin="$here/bin/devagotchi-bridge-linux-$arch"
(cd "$here/bin" && sha256sum -c --ignore-missing SHA256SUMS) || { echo "checksum mismatch"; exit 1; }
mkdir -p "$HOME/.local/bin" && install -m 0755 "$bin" "$HOME/.local/bin/devagotchi-bridge"
dst="$HOME/.local/bin/devagotchi-bridge"
echo "Machine id in the hub: $MACHINE_ID ($MACHINE_LABEL). Get its code: hub > Machines > Add machine (or Re-enroll)."
read -rsp "Enrollment code: " code; echo
printf '%s\n' "$code" | "$dst" enroll --hub "$HUB_URL"; unset code
"$dst" probe || echo "probe reported problems (see above); the hub will show the sandbox state"
"$dst" install
if systemctl --user show-environment >/dev/null 2>&1; then
  systemctl --user daemon-reload && systemctl --user enable --now devagotchi-bridge
  loginctl enable-linger "$USER" 2>/dev/null || echo "note: enable lingering to keep it running after logout"
  echo "running: systemctl --user status devagotchi-bridge"
else
  echo "no systemd user bus: start it with  nohup $dst run >~/.devagotchi-bridge.log 2>&1 &"
fi
