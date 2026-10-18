#!/usr/bin/env bash
# Cross-compile the Devagotchi bridge for every supported OS/arch into dist/.
# Linux carries the full X11 GUI helper; macOS/Windows GUI backends are documented stubs in this build.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
out=$root/dist
mkdir -p "$out"
cd "$root/bridge"
for t in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
  os=${t%/*}; arch=${t#*/}; ext=""
  [ "$os" = windows ] && ext=".exe"
  GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X devagotchi/bridge/buildinfo.Version=${VERSION:-$(git describe --tags --always 2>/dev/null || echo dev)}" \
    -o "$out/devagotchi-bridge-$os-$arch$ext" ./cmd/devagotchi-bridge
  echo "built devagotchi-bridge-$os-$arch$ext"
done
