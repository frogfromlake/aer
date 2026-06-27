#!/usr/bin/env bash
#
# Phase 150 — ILM (data-retention) live enforcement check. The MinIO bucket
# lifecycle rules + ClickHouse table TTLs are CONFIGURED in IaC, but configured
# is not the same as enforcing: a broken lifecycle scan or a stalled TTL merge
# would let data grow past its retention silently — unbounded storage growth AND
# a DSGVO right-to-erasure breach. This script VERIFIES enforcement against the
# real data and emits a Prometheus heartbeat so the guardrail is continuous, not
# a one-off: it writes aer_ilm_* via the node-exporter textfile collector (same
# pattern as backup.sh), which the ILMCheckStale / ILMViolation alerts read.
#
# Runs on the box, reachable on the aer-backend network. Invoke by hand
# (`bash scripts/operations/verify_ilm.sh`) or via the aer-verify-ilm systemd
# timer (see infra/systemd/). Exits non-zero if any TTL is being violated.
#
# It is a READ-ONLY check — it never deletes anything; it only reports whether
# the configured ILM is actually pruning. Lazy expiry (MinIO's scanner, ClickHouse
# background merges) means freshly-expired data can linger briefly, so each TTL
# carries a small grace window (ILM_GRACE_DAYS, default 2) to avoid false alarms.
set -euo pipefail

# Load .env the way Docker Compose does (flat KEY=VALUE, value verbatim) rather
# than shell-sourcing it — the WEB_CRAWLER_USER_AGENT value carries unescaped
# parens/semicolons that break `. ./.env` (mirrors backup.sh / reset_validate.sh).
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

# Phase 155 / ADR-046: in production, secrets are staged to a tmpfs dir at deploy
# time and override any .env value loaded above. No-op when the dir is absent.
# shellcheck source=scripts/operations/_secret_lib.sh
. "$(dirname "$0")/_secret_lib.sh"
load_secret_dir

: "${CLICKHOUSE_USER:?}"
: "${CLICKHOUSE_PASSWORD:?}"
: "${CLICKHOUSE_DB:?}"
: "${MINIO_ROOT_USER:?}"
: "${MINIO_ROOT_PASSWORD:?}"

GRACE_DAYS="${ILM_GRACE_DAYS:-2}"
TEXTFILE_DIR="${BACKUP_TEXTFILE_DIR:-/var/lib/aer/textfile}"

dc() { docker compose "$@"; }

violations=0

# --- heartbeat metric (atomic write, same convention as backup.sh) -----------
write_metric() {
  local v="$1" now; now="$(date +%s)"
  mkdir -p "$TEXTFILE_DIR"
  local tmp="$TEXTFILE_DIR/.aer_ilm.prom.$$"
  {
    echo "# HELP aer_ilm_last_check_timestamp_seconds Unix time of the last ILM enforcement check."
    echo "# TYPE aer_ilm_last_check_timestamp_seconds gauge"
    echo "aer_ilm_last_check_timestamp_seconds ${now}"
    echo "# HELP aer_ilm_violations Number of layers (MinIO bucket / ClickHouse table) holding data past its TTL+grace."
    echo "# TYPE aer_ilm_violations gauge"
    echo "aer_ilm_violations ${v}"
  } > "$tmp"
  mv "$tmp" "$TEXTFILE_DIR/aer_ilm.prom"
}
# Always emit the heartbeat, even on an early failure, so ILMCheckStale stays
# accurate; the violation count is whatever we counted before exiting.
trap 'write_metric "$violations"' EXIT

echo "==> AĒR ILM enforcement check (grace ${GRACE_DAYS}d)"

# --- ClickHouse Gold TTLs ----------------------------------------------------
# "<table>:<anchor-column>:<ttl-days>" — anchors + TTLs per CLAUDE.md / the Gold
# migrations. The raw tables expire at 365 d; the AggregatingMergeTree resolution
# MVs (metrics_hourly/daily/monthly) intentionally retain LONGER and are excluded.
ch_check() {
  local table="$1" anchor="$2" ttl="$3" cutoff=$(( $3 + GRACE_DAYS )) n
  n="$(dc exec -T clickhouse clickhouse-client \
        -u "$CLICKHOUSE_USER" --password "$CLICKHOUSE_PASSWORD" \
        -q "SELECT count() FROM ${CLICKHOUSE_DB}.${table} WHERE ${anchor} < now() - INTERVAL ${cutoff} DAY" \
        2>/dev/null | tr -d '[:space:]')"
  n="${n:-0}"
  if [ "$n" -gt 0 ]; then
    echo "  ✗ ClickHouse ${table}: ${n} rows older than ${ttl}d (+${GRACE_DAYS}d grace) — TTL not enforcing"
    violations=$(( violations + 1 ))
  else
    echo "  ✓ ClickHouse ${table}: no rows past ${ttl}d"
  fi
}
for spec in \
  "metrics:timestamp:365" \
  "entities:timestamp:365" \
  "entity_links:timestamp:365" \
  "language_detections:timestamp:365" \
  "entity_cooccurrences:window_start:365" \
  "topic_assignments:window_start:365"; do
  IFS=: read -r t a d <<< "$spec"
  ch_check "$t" "$a" "$d"
done

# --- MinIO bucket lifecycles -------------------------------------------------
# `mc find --older-than` does the date arithmetic INSIDE mc (the minio/mc image
# is minimal — no grep/awk/wc), so the container only emits matching object
# paths; the host counts the lines. Any object older than TTL+grace = a bucket
# whose expiry lifecycle is not pruning.
minio_check() {
  local bucket="$1" ttl="$2" cutoff=$(( $2 + GRACE_DAYS )) out n
  out="$(dc run --rm --no-deps -T --entrypoint /bin/sh minio-init -c "
    set -eu
    mc alias set src \"http://\${MINIO_ENDPOINT:-minio:9000}\" \"\$MINIO_ROOT_USER\" \"\$MINIO_ROOT_PASSWORD\" >/dev/null
    mc find \"src/${bucket}\" --older-than ${cutoff}d
  " 2>/dev/null || true)"
  n="$(printf '%s\n' "$out" | grep -c . || true)"
  if [ "${n:-0}" -gt 0 ]; then
    echo "  ✗ MinIO ${bucket}: ${n} objects older than ${ttl}d (+${GRACE_DAYS}d grace) — lifecycle not pruning"
    violations=$(( violations + 1 ))
  else
    echo "  ✓ MinIO ${bucket}: no objects past ${ttl}d"
  fi
}
minio_check "bronze" 90
minio_check "silver" 365
minio_check "bronze-quarantine" 30

# --- verdict -----------------------------------------------------------------
if [ "$violations" -gt 0 ]; then
  echo "==> ILM check FAILED: ${violations} layer(s) holding data past its TTL. Investigate the MinIO lifecycle config + ClickHouse TTL merges."
  exit 1
fi
echo "==> ILM check passed: every layer is within its retention bound."
