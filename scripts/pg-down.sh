#!/usr/bin/env bash
# Stops the box-only Postgres cluster started by pg-up.sh. Touches only .run/pg.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
bin=${PG_BIN:-$(ls -d /usr/lib/postgresql/*/bin | sort -V | tail -1)}
"$bin/pg_ctl" -D "$root/.run/pg" -m fast stop
