#!/usr/bin/env bash
# Build the single devagotchi binary for every target: static (CGO off), stripped, version embedded.
# Output: dist/release/devagotchi-<os>-<arch>[.exe], SHA256SUMS, sizes.txt. Builds only; installs nothing.
# Data knobs: TARGETS="linux/amd64 ..." VERSION=... OUT=...
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
out=${OUT:-$root/dist/release}
targets=${TARGETS:-"linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64"}
cd "$root/bridge"
version=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}
commit=$(git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
date=$(date -u +%Y-%m-%dT%H:%M:%SZ)
pkg=devagotchi/bridge/buildinfo
ldflags="-s -w -buildid= -X $pkg.Version=$version -X $pkg.Commit=$commit -X $pkg.Date=$date"
rm -rf "$out" && mkdir -p "$out"
for t in $targets; do
  os=${t%/*}; arch=${t#*/}; ext=""
  [ "$os" = windows ] && ext=".exe"
  name="devagotchi-$os-$arch$ext"
  GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -trimpath -ldflags="$ldflags" -o "$out/$name" ./cmd/devagotchi
done
cd "$out"
sha256sum devagotchi-* > SHA256SUMS
{
  printf '%-32s %10s %10s\n' file bytes gzip_bytes
  for f in devagotchi-*; do printf '%-32s %10s %10s\n' "$f" "$(stat -c %s "$f")" "$(gzip -9c "$f" | wc -c)"; done
} > sizes.txt
cat sizes.txt
echo "version $version ($commit) -> $out"
