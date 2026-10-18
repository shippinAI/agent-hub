#!/bin/sh
# Install the devagotchi binary for this user (Linux/macOS). No sudo, no service, no telemetry.
#   curl -fsSL "$BASE/install.sh" | DEVAGOTCHI_BASE_URL="$BASE" sh
#   sh install.sh --from ./dist/release          (offline, from a release directory)
# Knobs (data): DEVAGOTCHI_BASE_URL, DEVAGOTCHI_FROM, DEVAGOTCHI_BIN_DIR (default ~/.local/bin).
set -eu
base=${DEVAGOTCHI_BASE_URL:-}
from=${DEVAGOTCHI_FROM:-}
bindir=${DEVAGOTCHI_BIN_DIR:-$HOME/.local/bin}
[ "${1:-}" = "--from" ] && from=$2
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case $(uname -m) in x86_64|amd64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; *) echo "unsupported CPU $(uname -m)" >&2; exit 1 ;; esac
case $os in linux|darwin) ;; *) echo "unsupported OS $os (use install.ps1 on Windows)" >&2; exit 1 ;; esac
name=devagotchi-$os-$arch
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
fetch() { # fetch NAME -> $tmp/NAME
  if [ -n "$from" ]; then cp "$from/$1" "$tmp/$1"
  elif [ -n "$base" ]; then curl -fsSL --proto '=https' --tlsv1.2 -o "$tmp/$1" "$base/$1" 2>/dev/null || curl -fsSL -o "$tmp/$1" "$base/$1"
  else echo "set DEVAGOTCHI_BASE_URL or pass --from DIR" >&2; exit 1; fi
}
fetch "$name"; fetch SHA256SUMS
want=$(grep " $name\$" "$tmp/SHA256SUMS" | cut -d' ' -f1)
if command -v sha256sum >/dev/null; then got=$(sha256sum "$tmp/$name" | cut -d' ' -f1); else got=$(shasum -a 256 "$tmp/$name" | cut -d' ' -f1); fi
[ -n "$want" ] && [ "$want" = "$got" ] || { echo "checksum mismatch for $name" >&2; exit 1; }
mkdir -p "$bindir"
install -m 0755 "$tmp/$name" "$bindir/devagotchi"
[ "$os" = darwin ] && xattr -d com.apple.quarantine "$bindir/devagotchi" 2>/dev/null || true
"$bindir/devagotchi" version
case ":$PATH:" in *":$bindir:"*) ;; *) echo "add $bindir to PATH" ;; esac
echo "next: devagotchi doctor; devagotchi config init --root ~/code"
