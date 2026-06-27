#!/usr/bin/env bash
#
# SEC-031 / SEC-047 / SEC-049 / SEC-050 — AĒR off-box backup of the four
# irreplaceable stores. Runs on the production host (where `docker compose` for
# the stack runs). The documented "wipe-and-recrawl" recovery covers NONE of
# these, so this is the only path back from a disk failure or an accidental wipe.
#
# What it captures:
#   1. Postgres `aer_metadata`  — auth (users + argon2id hashes + consent),
#      sessions, single-use tokens, saved_analyses, crawler_state cursors,
#      sources/documents metadata. (pg_dump custom format, one consistent dump.)
#   2. ClickHouse `aer_gold`    — native BACKUP, which captures the
#      AggregatingMergeTree resolution-MV partitions DIRECTLY (SEC-047): the
#      indefinite "climate record" is unrebuildable by replay once raw metrics
#      age past the 365d TTL, so we snapshot the aggregate parts, never replay.
#   3. MinIO bronze + silver    — raw HTML (the only Silver/Gold replay source)
#      and refined Silver, mirrored logically via mc.
#
# Transport + encryption + retention: a single restic repository on an in-EU
# Hetzner Storage Box (D1–D3). restic does client-side encryption (a leaked
# provider cannot read backups), dedup, and retention prune. The 35d window
# (D2) bounds DSGVO-erasure persistence — an erased user ages out of backups.
#
# Heartbeat: writes Prometheus textfile metrics consumed by node-exporter so a
# stale/failed backup is alertable (BackupStale / BackupFailed — SEC-049/053).
#
# HOST PREREQUISITES (documented in docs/operations/backup_restore.md):
#   - `restic` installed on the host
#   - the host's SSH configured for the Storage Box (key + known_hosts), so the
#     `sftp:` restic repository is reachable non-interactively
#   - run with the stack env loaded (e.g. `make backup`, which loads .env), and
#     for a prod stack set `COMPOSE_FILE=compose.yaml:compose.prod.yaml`.
#
# Live validation: this script MUST be exercised end-to-end (backup + a test
# restore into a throwaway stack — see restore.sh) before it is relied upon.
#
set -euo pipefail

# Load .env WITHOUT shell-sourcing it. The WEB_CRAWLER_USER_AGENT value carries
# unescaped parens + a semicolon (WP-006 §5.1), so `. ./.env` parse-errors —
# which is why this reads the file the way Docker Compose does: flat KEY=VALUE,
# value verbatim to end-of-line, exporting only well-formed identifiers (mirrors
# scripts/operations/reset_validate.sh). Self-loading means this works the same
# whether invoked by `make backup`, the systemd timer, or directly; values already
# in the environment (e.g. COMPOSE_FILE from the unit) survive unless .env sets them.
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

# --- required configuration (fail fast, never silently skip a store) ---------
: "${RESTIC_REPOSITORY:?set RESTIC_REPOSITORY, e.g. sftp:u123456@u123456.your-storagebox.de:/home/aer-backups (path MUST be under /home — a Storage Box only allows writes there; an absolute /aer-backups fails restic init with SSH_FX_FAILURE)}"
: "${RESTIC_PASSWORD:?set RESTIC_PASSWORD (client-side encryption key — ESCROW OFF-BOX; losing it = unrecoverable backups)}"
: "${POSTGRES_USER:?}"
: "${POSTGRES_PASSWORD:?}"
: "${POSTGRES_DB:?}"
: "${CLICKHOUSE_USER:?}"
: "${CLICKHOUSE_PASSWORD:?}"
: "${CLICKHOUSE_DB:?}"
: "${MINIO_ROOT_USER:?}"
: "${MINIO_ROOT_PASSWORD:?}"

RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-35}"
TEXTFILE_DIR="${BACKUP_TEXTFILE_DIR:-/var/lib/aer/textfile}"

dc() { docker compose "$@"; }

TS_START="$(date +%s)"
CH_BACKUP_NAME="aer_gold_$(date -u +%Y%m%dT%H%M%SZ)"
STAGING="$(mktemp -d)"
status=1   # assume failure until the end; the EXIT trap reports it

# --- heartbeat metrics -------------------------------------------------------
# Two separate textfiles so a FAILED run never erases the last-success series
# (BackupStale must still measure age against the last good backup).
write_run_metric() {
  local code="$1" now; now="$(date +%s)"
  mkdir -p "$TEXTFILE_DIR"
  local tmp="$TEXTFILE_DIR/.aer_backup_run.prom.$$"
  {
    echo "# HELP aer_backup_last_run_timestamp_seconds Unix time of the last backup attempt."
    echo "# TYPE aer_backup_last_run_timestamp_seconds gauge"
    echo "aer_backup_last_run_timestamp_seconds ${now}"
    echo "# HELP aer_backup_last_exit_code Exit code of the last backup run (0 = success)."
    echo "# TYPE aer_backup_last_exit_code gauge"
    echo "aer_backup_last_exit_code ${code}"
  } > "$tmp"
  mv "$tmp" "$TEXTFILE_DIR/aer_backup_run.prom"
}

write_success_metric() {
  local now; now="$(date +%s)"
  mkdir -p "$TEXTFILE_DIR"
  local tmp="$TEXTFILE_DIR/.aer_backup_success.prom.$$"
  {
    echo "# HELP aer_backup_last_success_timestamp_seconds Unix time of the last SUCCESSFUL backup."
    echo "# TYPE aer_backup_last_success_timestamp_seconds gauge"
    echo "aer_backup_last_success_timestamp_seconds ${now}"
    echo "# HELP aer_backup_last_duration_seconds Wall-clock duration of the last successful backup."
    echo "# TYPE aer_backup_last_duration_seconds gauge"
    echo "aer_backup_last_duration_seconds $(( now - TS_START ))"
  } > "$tmp"
  mv "$tmp" "$TEXTFILE_DIR/aer_backup_success.prom"
}

finish() {
  write_run_metric "$status"
  rm -rf "$STAGING"
}
trap finish EXIT

echo "==> AĒR backup starting (staging: $STAGING)"

# --- restic repository (init on first run) -----------------------------------
if ! restic cat config >/dev/null 2>&1; then
  echo "==> Initialising restic repository at ${RESTIC_REPOSITORY}"
  restic init
fi

# --- 1. Postgres (auth + crawler_state + metadata, one consistent dump) ------
echo "==> Dumping Postgres ${POSTGRES_DB}"
mkdir -p "$STAGING/postgres"
dc exec -T -e PGPASSWORD="$POSTGRES_PASSWORD" postgres \
  pg_dump -U "$POSTGRES_USER" --format=custom "$POSTGRES_DB" \
  > "$STAGING/postgres/${POSTGRES_DB}.dump"

# --- 2. ClickHouse (native BACKUP of aer_gold incl. AMT MV partitions) -------
echo "==> Backing up ClickHouse ${CLICKHOUSE_DB} (native, AMT partitions direct)"
dc exec -T clickhouse clickhouse-client \
  -u "$CLICKHOUSE_USER" --password "$CLICKHOUSE_PASSWORD" \
  -q "BACKUP DATABASE ${CLICKHOUSE_DB} TO Disk('backups', '${CH_BACKUP_NAME}')"
mkdir -p "$STAGING/clickhouse"
dc exec -T clickhouse tar -C /backups -cf - "${CH_BACKUP_NAME}" \
  > "$STAGING/clickhouse/${CH_BACKUP_NAME}.tar"
# Prune the in-volume copy so clickhouse_backups does not grow unbounded.
dc exec -T clickhouse rm -rf "/backups/${CH_BACKUP_NAME}"

# --- 3. MinIO bronze + silver (logical mirror via mc, in-network) ------------
echo "==> Mirroring MinIO bronze + silver"
mkdir -p "$STAGING/minio"
dc run --rm --no-deps -T -v "$STAGING/minio:/mirror" \
  --entrypoint /bin/sh minio-init -c '
    set -eu
    mc alias set src "http://${MINIO_ENDPOINT:-minio:9000}" "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null
    mc mirror --overwrite --remove src/bronze /mirror/bronze
    mc mirror --overwrite --remove src/silver /mirror/silver
  '

# --- 4. restic: encrypted off-box snapshot + retention prune -----------------
echo "==> Uploading encrypted snapshot to ${RESTIC_REPOSITORY}"
restic backup --tag aer --host aer-prod "$STAGING"

echo "==> Pruning snapshots older than ${RETENTION_DAYS}d"
# --group-by host (NOT the default host,paths): each run stages into a fresh
# mktemp dir, so the backup path differs every time. With the default grouping
# every run would form its own single-snapshot group, and --keep-within would
# then keep "the newest in the group = itself" forever — silently breaking the
# 35d retention (the DSGVO right-to-erasure bound, D2) and growing the repo
# unbounded. Grouping by host alone collapses all aer-prod snapshots into one
# group so the age policy actually prunes.
restic forget --tag aer --group-by host --keep-within "${RETENTION_DAYS}d" --prune

write_success_metric
status=0
echo "==> AĒR backup complete in $(( $(date +%s) - TS_START ))s"
