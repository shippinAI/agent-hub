#!/usr/bin/env bash
# Guide-driven Podman verification, box-only. Runs the executable linux steps of the seeded podman-setup
# guide (present -> verify) inside the disposable Debian rootfs, so the container runtime never touches the
# box. The commands come straight from the guide seed so this proves the shipped guide actually works.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
R=$root/.run/disposable/try1
guide=$root/server/internal/presets/guides/podman-setup.json
ev=$root/test-evidence/guide-podman-run.txt
[ -d "$R" ] || { echo "disposable rootfs missing at $R (build it first)"; exit 1; }

present=$(python3 -c "import json;g=json.load(open('$guide'));[print(' '.join(s['args']['argv'])) for s in g['version']['steps'] if s['id']=='present']")
verify=$(python3 -c "import json;g=json.load(open('$guide'));[print(' '.join(s['args']['argv'])) for s in g['version']['steps'] if s['id']=='verify']")

opts="--storage-driver=vfs --network=host --cgroups=disabled"
# the guide's verify is 'podman run --rm quay.io/podman/hello'; add the box-only storage/network opts
verify=${verify/podman run/podman run $opts}

{
  echo "# podman-setup guide, linux steps, in disposable container  $(date '+%F %T %Z')"
  echo "## present step: $present"
  sudo "$root/tests/disposable/ct-enter.sh" "$R" bash -lc "$present" 2>&1 | sed 's/^/  /'
  echo "## verify step:  $verify"
  sudo "$root/tests/disposable/ct-enter.sh" "$R" bash -lc "$verify" 2>&1 | sed 's/^/  /'
  echo "## OK: guide steps ran in the disposable container; the box host was not touched."
} | tee "$ev"
