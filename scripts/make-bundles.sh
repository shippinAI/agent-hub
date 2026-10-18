#!/usr/bin/env bash
# Build per-host bridge install bundles into dist/bundles/<host>/ (+ .tar.gz / .zip). Builds only; installs nothing.
# Usage: scripts/make-bundles.sh [hub-url]   (default: HUB_URL left empty until the box is on the tailnet)
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
src=$root/deploy/bundles out=$root/dist/bundles
"$root/scripts/build-bridges.sh" >/dev/null
rm -rf "$out" && mkdir -p "$out"
for envf in "$src"/hosts/*.env; do
  id=$(basename "$envf" .env); os=$(sed -n 's/^OS=//p' "$envf"); d=$out/$id
  mkdir -p "$d/bin"
  sed "s|^HUB_URL=.*|HUB_URL=${1:-}|" "$envf" > "$d/host.env"
  if [ "$os" = windows ]; then
    cp "$root"/dist/devagotchi-bridge-windows-{amd64,arm64}.exe "$d/bin/"; cp "$src/install.ps1" "$d/"
    one='powershell -ExecutionPolicy Bypass -File .\install.ps1   (elevated)'
  else
    cp "$root"/dist/devagotchi-bridge-linux-{amd64,arm64} "$d/bin/"; install -m 0755 "$src/install.sh" "$d/"
    one='./install.sh'
  fi
  (cd "$d/bin" && sha256sum devagotchi-bridge-* > SHA256SUMS)
  sed -e "s|@ID@|$id|g" -e "s|@ONE@|$one|g" "$src/README.tmpl.md" > "$d/README.md"
  if [ "$os" = windows ]; then (cd "$out" && rm -f "$id.zip" && python3 -m zipfile -c "$id.zip" "$id" >/dev/null)
  else tar -C "$out" -czf "$out/$id.tar.gz" "$id"; fi
  echo "bundle $id ($os): $d"
done
