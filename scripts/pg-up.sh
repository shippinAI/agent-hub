#!/usr/bin/env bash
# Box-only Postgres for agent-hub: a private cluster under .run/pg (no systemd, no system cluster).
# Touches: .run/pg (data), .run/secrets/db-password + db-url (0600). Listens on 127.0.0.1:${HUB_PG_PORT:-54329}.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd); run=$root/.run
bin=${PG_BIN:-$(ls -d /usr/lib/postgresql/*/bin | sort -V | tail -1)}
port=${HUB_PG_PORT:-54329}
mkdir -p "$run/secrets" && chmod 700 "$run/secrets"
if [ ! -s "$run/pg/PG_VERSION" ]; then
  "$bin/initdb" -D "$run/pg" -U postgres --auth-local=peer --auth-host=scram-sha-256 >/dev/null
  printf "listen_addresses='127.0.0.1'\nport=%s\nunix_socket_directories='%s'\n" "$port" "$run" >> "$run/pg/postgresql.conf"
  # peer auth maps the OS user to the postgres superuser on the socket only
  printf "local all postgres peer map=box\n" > "$run/pg/pg_hba.local"
  cat "$run/pg/pg_hba.local" "$run/pg/pg_hba.conf" > "$run/pg/hba.tmp" && mv "$run/pg/hba.tmp" "$run/pg/pg_hba.conf"
  printf "box %s postgres\n" "$(id -un)" >> "$run/pg/pg_ident.conf"
fi
if ! "$bin/pg_ctl" -D "$run/pg" status >/dev/null 2>&1; then
  "$bin/pg_ctl" -D "$run/pg" -l "$run/pg.log" -w start >/dev/null
fi
if [ ! -s "$run/secrets/db-password" ]; then
  (umask 077; head -c 24 /dev/urandom | base64 | tr -d '/+=' > "$run/secrets/db-password")
fi
pw=$(cat "$run/secrets/db-password")
psql=("$bin/psql" -h "$run" -p "$port" -U postgres -v ON_ERROR_STOP=1 -qAt)
for db in ${HUB_PG_DBS:-agenthub agenthub_test}; do
  "${psql[@]}" -d postgres -c "DO \$\$BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='hub') THEN CREATE ROLE hub LOGIN; END IF; END\$\$;" >/dev/null
  "${psql[@]}" -d postgres -c "ALTER ROLE hub PASSWORD '$pw';" >/dev/null
  "${psql[@]}" -d postgres -tc "SELECT 1 FROM pg_database WHERE datname='$db'" | grep -q 1 \
    || "${psql[@]}" -d postgres -c "CREATE DATABASE $db OWNER hub;" >/dev/null
  (umask 077; printf 'postgres://hub:%s@127.0.0.1:%s/%s?sslmode=disable\n' "$pw" "$port" "$db" > "$run/secrets/db-url-$db")
done
echo "postgres up on 127.0.0.1:$port (urls in .run/secrets/db-url-*, not printed)"
