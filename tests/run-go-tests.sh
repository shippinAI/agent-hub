#!/usr/bin/env bash
# Go unit + integration tests against the box-only Postgres (agenthub_test DB is wiped per test).
# Touches: .run/pg (starts it if needed), agenthub_test DB, test-evidence/go-tests.txt.
set -uo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
"$root/scripts/pg-up.sh" >/dev/null
out=$root/test-evidence/go-tests.txt
cd "$root/server"
{
  echo "# go test $(date '+%Y-%m-%d %H:%M %Z')"
  go vet ./... && echo "go vet: ok"
  HUB_TEST_DATABASE_URL_FILE=$root/.run/secrets/db-url-agenthub_test go test -count=1 -p 1 -v ./... 2>&1 \
    | grep -E '^(=== RUN|--- |    |PASS|FAIL|ok|panic)'
} | tee "$out"
grep -q -- '--- FAIL\|^FAIL' "$out" && exit 1 || exit 0
