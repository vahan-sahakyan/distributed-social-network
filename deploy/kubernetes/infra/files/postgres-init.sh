#!/bin/sh
# creates one database per service on the instance's first start; shared by compose and the
# infra chart. DATABASES: space-separated names
set -eu
for db in $DATABASES; do
  psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d postgres -c "CREATE DATABASE \"$db\""
done
