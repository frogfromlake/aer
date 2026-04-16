#!/bin/sh
# init-roles.sh — provisions the BFF API's read-only PostgreSQL role.
#
# Hard Rule 5 (IaC-only provisioning): services do not create their own
# database roles or grants at startup. This init container creates the
# `bff_readonly` role idempotently and grants SELECT on the `sources`
# table so the BFF API can serve /api/v1/sources directly from the SSoT
# Postgres table instead of a mirrored YAML file (Phase 87).
#
# The script waits for the `sources` table to exist because the
# ingestion-api runs the schema migrations on its own startup and may
# create the table slightly after this container begins. The wait is
# bounded so a misconfigured stack fails loudly rather than hanging.

set -eu

: "${PGHOST:?PGHOST is required}"
: "${PGUSER:?PGUSER is required}"
: "${PGPASSWORD:?PGPASSWORD is required}"
: "${PGDATABASE:?PGDATABASE is required}"
: "${BFF_DB_USER:?BFF_DB_USER is required}"
: "${BFF_DB_PASSWORD:?BFF_DB_PASSWORD is required}"

export PGHOST PGUSER PGPASSWORD PGDATABASE

echo "postgres-init-roles: waiting for sources table to be created by ingestion-api migrations..."
ATTEMPTS=60
while [ "$ATTEMPTS" -gt 0 ]; do
    if psql -v ON_ERROR_STOP=1 -tAc "SELECT to_regclass('public.sources') IS NOT NULL" | grep -q '^t$'; then
        break
    fi
    ATTEMPTS=$((ATTEMPTS - 1))
    sleep 1
done

if [ "$ATTEMPTS" -eq 0 ]; then
    echo "postgres-init-roles: sources table did not appear in time — aborting" >&2
    exit 1
fi

echo "postgres-init-roles: ensuring role '${BFF_DB_USER}' exists with read-only access to sources..."

# Idempotent role provisioning. The role is created via \gexec so psql
# variable interpolation works (DO $$ blocks suppress interpolation).
# The password is always (re)synced via ALTER ROLE so rotating
# BFF_DB_PASSWORD in .env is picked up on the next stack restart.
psql -v ON_ERROR_STOP=1 \
     -v bff_user="${BFF_DB_USER}" \
     -v bff_password="${BFF_DB_PASSWORD}" \
     -v pg_database="${PGDATABASE}" \
     <<'SQL'
SELECT format('CREATE ROLE %I LOGIN PASSWORD %L', :'bff_user', :'bff_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'bff_user') \gexec

ALTER ROLE :"bff_user" WITH LOGIN PASSWORD :'bff_password';

GRANT CONNECT ON DATABASE :"pg_database" TO :"bff_user";
GRANT USAGE ON SCHEMA public TO :"bff_user";
GRANT SELECT ON TABLE public.sources TO :"bff_user";

-- Revoke any incidental privileges that may have accumulated on other
-- public tables so the role stays narrowly scoped to the `sources` SSoT.
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM :"bff_user";
GRANT SELECT ON TABLE public.sources TO :"bff_user";
SQL

echo "postgres-init-roles: done."
