#!/bin/bash
# cloud/supabase/schema.sql in a real, throwaway Postgres (ROADMAP 2.10,
# 2.13): loads ci/supabase_stub.sql (Supabase's auth and storage, the
# little the schema uses), the schema twice (it must be re-runnable), then
# ci/supabase_schema_test.sql (accounts' rights through row-level
# security). Needs Postgres's server programs
# (PG_BIN, default the newest /usr/lib/postgresql/*/bin); run as a user
# Postgres may run as (not root).
#
#   bash ci/supabase_schema_test.sh
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
PG_BIN=${PG_BIN:-$(ls -d /usr/lib/postgresql/*/bin | sort -V | tail -1)}
dir=$(mktemp -d)
trap '"$PG_BIN/pg_ctl" -D "$dir/data" -m immediate stop >/dev/null 2>&1 || true; rm -rf "$dir"' EXIT
"$PG_BIN/initdb" -D "$dir/data" -A trust -U postgres >/dev/null
"$PG_BIN/pg_ctl" -D "$dir/data" -o "-k $dir -p 54329 -c listen_addresses=" -l "$dir/log" start >/dev/null
psql=("$PG_BIN/psql" -h "$dir" -p 54329 -U postgres -q -v ON_ERROR_STOP=1)
"${psql[@]}" -c "create database kc" postgres
"${psql[@]}" -f "$here/supabase_stub.sql" kc
for _ in 1 2; do
  "${psql[@]}" -f "$here/../cloud/supabase/schema.sql" kc 2>&1 | grep -vE 'does not exist, skipping|already exists, skipping' || true
done
"${psql[@]}" -t -A -f "$here/supabase_schema_test.sql" kc 2>&1 | sed 's/^psql:[^ ]* NOTICE:  //' |
  grep -vE '^$|^[0-9a-f-]{36}$|^[0-9]+$'
