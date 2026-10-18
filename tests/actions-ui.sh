#!/usr/bin/env bash
# Action feed + audit retention UI pass: a Go test drives a REAL bridge (bwrap sandbox, box-only) through
# denied / allowed / failing / editing calls on a throwaway test hub, holds it open, and headless Chrome
# screenshots the feed and the retention admin at 1280 and 390. Touches: agenthub_test DB (wiped),
# .run/ui/actions-demo.json (0600, removed after), test-evidence/screenshots/actions-*, test-evidence/actions-ui.txt.
set -uo pipefail
root=$(cd "$(dirname "$0")/.." && pwd); ev=$root/test-evidence; demo=$root/.run/ui/actions-demo.json
mkdir -p "$root/.run/ui" "$ev/screenshots" && rm -f "$demo"
"$root/scripts/pg-up.sh" >/dev/null
(cd "$root/web" && npm run build >/dev/null) || { echo "FAIL web build"; exit 1; }
(cd "$root/server" && HUB_UI_DEMO_HOLD=${HOLD:-60} HUB_UI_DEMO_FILE=$demo \
  HUB_TEST_DATABASE_URL_FILE=$root/.run/secrets/db-url-agenthub_test \
  go test -count=1 -run TestActionFeedDemo ./hubtest/ > /tmp/actions-demo.log 2>&1) &
for i in $(seq 1 240); do [ -s "$demo" ] && break; sleep 0.5; done
[ -s "$demo" ] || { echo "FAIL demo hub did not start"; cat /tmp/actions-demo.log; exit 1; }
(cd "$root/tests/ui" && node actions-shots.mjs "$demo" "$ev/screenshots" "$ev/actions-ui.txt")
pass=$(grep -c '^PASS' "$ev/actions-ui.txt"); fail=$(grep -c '^FAIL' "$ev/actions-ui.txt")
echo "actions-ui: $pass pass, $fail fail"
kill %1 2>/dev/null; wait 2>/dev/null
[ "$fail" = 0 ]
