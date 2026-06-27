#!/usr/bin/env bash
#
# SEC-048 — AĒR restore from the latest restic snapshot, in the corruption-safe
# order documented in docs/operations/backup_restore.md §5. Primarily for the
# mandatory pre-launch TEST restore into a throwaway stack, and for real
# disaster recovery.
#
# DESTRUCTIVE: overwrites Postgres, ClickHouse, and MinIO with the snapshot.
# Refuses to run unless RESTORE_CONFIRM=yes is set.
#
# Requires the same host prerequisites + env as backup.sh (restic, SSH to the
# Storage Box, stack env loaded). For a prod stack set
# COMPOSE_FILE=compose.yaml:compose.prod.yaml.
#
set -euo pipefail

# Load .env safely (NOT `. ./.env` — the WEB_CRAWLER_USER_AGENT parens parse-
# error; mirrors backup.sh / reset_validate.sh). Self-loading so this works from
# the systemd context or a manual incident shell alike.
load_env() {
  env_file="$1"
  [ -f "$env_file" ] || return 0
  while IFS= read -r _line || [ -n "$_line" ]; do
    case "$_line" in ''|\#*) continue ;; esac
    [ -z "${_line// }" ] && continue
    if [[ "$_line" =~ ^([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]]; then
      export "${BASH_REMATCH[1]}=${BASH_REMATCH[2]}"
    fi
  done < "$env_file"
  unset _line
}
load_env "$(cd "$(dirname "$0")/../.." && pwd)/.env"

: "${RESTIC_REPOSITORY:?}"
: "${RESTIC_PASSWORD:?}"
: "${POSTGRES_USER:?}"; : "${POSTGRES_PASSWORD:?}"; : "${POSTGRES_DB:?}"
: "${CLICKHOUSE_USER:?}"; : "${CLICKHOUSE_PASSWORD:?}"; : "${CLICKHOUSE_DB:?}"
: "${MINIO_ROOT_USER:?}"; : "${MINIO_ROOT_PASSWORD:?}"

if [ "${RESTORE_CONFIRM:-}" != "yes" ]; then
  echo "REFUSING: restore is destructive (overwrites PG + ClickHouse + MinIO)." >&2
  echo "Re-run with RESTORE_CONFIRM=yes once you are sure (and on the right stack)." >&2
  exit 1
fi

dc() { docker compose "$@"; }
STAGING="$(mktemp -d)"
trap 'rm -rf "$STAGING"' EXIT

echo "==> 0. Pausing the pipeline (no writes during restore)"
dc stop web-crawler analysis-worker ingestion-api || true

echo "==> 2. Pulling latest restic snapshot"
restic restore latest --tag aer --target "$STAGING"
# restic restores the absolute backup path; locate the captured staging subtree.
SNAP_ROOT="$(dirname "$(find "$STAGING" -type d -name postgres | head -1)")"
[ -n "$SNAP_ROOT" ] || { echo "could not locate snapshot contents" >&2; exit 1; }

echo "==> 3. Restoring Postgres ${POSTGRES_DB}"
dc exec -T -e PGPASSWORD="$POSTGRES_PASSWORD" postgres \
  pg_restore -U "$POSTGRES_USER" --clean --if-exists -d "$POSTGRES_DB" \
  < "${SNAP_ROOT}/postgres/${POSTGRES_DB}.dump"
echo "    re-applying Postgres roles (grants live in init-roles.sh, not the dump)"
dc up -d postgres-init-roles || echo "    (re-run postgres-init-roles manually if absent)"

echo "==> 4. Restoring ClickHouse ${CLICKHOUSE_DB} (native RESTORE + OPTIMIZE FINAL)"
CH_TAR="$(find "${SNAP_ROOT}/clickhouse" -name '*.tar' | head -1)"
CH_NAME="$(tar -tf "$CH_TAR" | head -1 | cut -d/ -f1)"
# Stream the tar straight into the clickhouse_backups volume: the archive holds
# <CH_NAME>/..., so extracting it under /backups yields /backups/<CH_NAME>/.
dc exec -T clickhouse sh -c "mkdir -p /backups && cd /backups && tar -xf -" < "$CH_TAR"
# RESTORE DATABASE fails ("table already exists") against a populated target.
# This script is DESTRUCTIVE by contract (RESTORE_CONFIRM=yes) and pg_restore
# above already uses --clean, so mirror that for ClickHouse: drop the database
# first so RESTORE recreates it cleanly. Works whether the target is empty (fresh
# DR / throwaway stack) OR holds stale/corrupt data (in-place recovery, test drill).
dc exec -T clickhouse clickhouse-client -u "$CLICKHOUSE_USER" --password "$CLICKHOUSE_PASSWORD" \
  -q "DROP DATABASE IF EXISTS ${CLICKHOUSE_DB}"
dc exec -T clickhouse clickhouse-client -u "$CLICKHOUSE_USER" --password "$CLICKHOUSE_PASSWORD" \
  -q "RESTORE DATABASE ${CLICKHOUSE_DB} FROM Disk('backups', '${CH_NAME}')"
# Collapse any duplicate ReplacingMergeTree parts before the pipeline resumes.
for t in metrics entities entity_links language_detections entity_cooccurrences topic_assignments; do
  dc exec -T clickhouse clickhouse-client -u "$CLICKHOUSE_USER" --password "$CLICKHOUSE_PASSWORD" \
    -q "OPTIMIZE TABLE ${CLICKHOUSE_DB}.${t} FINAL" 2>/dev/null || true
done
dc exec -T clickhouse sh -c "rm -rf /backups/${CH_NAME}" || true

echo "==> 5. Mirroring MinIO bronze + silver back"
dc run --rm --no-deps -T -v "${SNAP_ROOT}/minio:/mirror" \
  --entrypoint /bin/sh minio-init -c '
    set -eu
    mc alias set dst "http://${MINIO_ENDPOINT:-minio:9000}" "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null
    mc mirror --overwrite /mirror/bronze dst/bronze
    mc mirror --overwrite /mirror/silver dst/silver
  '

echo "==> 6. Resuming the pipeline"
dc start ingestion-api analysis-worker web-crawler

echo "==> Restore complete. Verify row counts + a sample login before declaring success."
