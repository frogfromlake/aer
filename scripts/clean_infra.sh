#!/bin/bash
# Wipes runtime state-bearing volumes, preserving build-time artefact volumes.
#
# Targets:
#   all            — wipe every runtime data volume (Phase-120b reset path)
#   postgres       — wipe Postgres data only
#   minio          — wipe MinIO data only
#   clickhouse     — wipe ClickHouse data only
#
# Volumes the `all` target WIPES (runtime / mixed-vintage data):
#   aer_postgres_data         — documents idempotency table + seed-migration state
#   aer_minio_data            — Bronze, Silver, Quarantine buckets
#   aer_clickhouse_data       — Gold + Silver-projection rows
#   aer_rss_crawler_state     — RSS dedup state (must travel with Bronze + documents)
#
# Volumes the `all` target PRESERVES (build-time artefacts; wasteful to refetch):
#   aer_wikidata_data         — Phase 118 Wikidata alias index (~190 MB)
#   aer_tempo_data            — Phase 80 observability traces (no operational impact, just retention)
#
# The Phase 119 BERT models + Phase 120 BERTopic embeddings (~10 GB) are
# baked into the worker image at /hf-cache, NOT held in a named volume —
# the worker runs with TRANSFORMERS_OFFLINE=1, so the image is the
# canonical store. Rebuilding the worker image is what protects model
# state across resets, not a Docker volume.
#
# The targeted modes (postgres / minio / clickhouse) NEVER touch the
# preserved volumes. Use them when only one storage layer needs wiping.
#
# Set AER_RESET_NONINTERACTIVE=1 to skip the confirmation prompt
# (used by `make reset` for one-shot supervised resets).

set -euo pipefail
TARGET="${1:-}"

# Colors
GOLD='\033[38;5;214m'
GREEN='\033[38;5;76m'
CYAN='\033[38;5;39m'
RED='\033[38;5;196m'
GRAY='\033[38;5;245m'
RESET='\033[0m'

PROJECT_NAME="aer"

# Volumes wiped by the `all` mode. Explicit allow-list — NOT
# `docker compose down -v`, which would also remove the preserved
# build-time volumes below.
RUNTIME_STATE_VOLUMES=(
    "${PROJECT_NAME}_postgres_data"
    "${PROJECT_NAME}_minio_data"
    "${PROJECT_NAME}_clickhouse_data"
    "${PROJECT_NAME}_rss_crawler_state"
)

# Volumes deliberately preserved across resets. Listed here for visibility
# and post-wipe assertion only — the script never deletes these names.
PRESERVED_VOLUMES=(
    "${PROJECT_NAME}_wikidata_data"
    "${PROJECT_NAME}_tempo_data"
)

confirm_deletion() {
    local label=$1
    echo -e "${GOLD}⚠️  WARNING: This will permanently delete: ${label}${RESET}"
    if [[ "${AER_RESET_NONINTERACTIVE:-0}" == "1" ]]; then
        echo -e "${CYAN}ℹ AER_RESET_NONINTERACTIVE=1 — proceeding without prompt.${RESET}"
        return
    fi
    read -p "Are you sure? (y/N): " confirm
    if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
        echo -e "${CYAN}ℹ Cancelled. No data deleted.${RESET}"
        exit 0
    fi
}

remove_volume() {
    local name=$1
    if docker volume inspect "$name" >/dev/null 2>&1; then
        docker volume rm "$name" >/dev/null
        echo -e "${GREEN}✔ removed: $name${RESET}"
    else
        echo -e "${GRAY}· already absent: $name${RESET}"
    fi
}

assert_preserved() {
    # Verify that the volumes we promised to preserve are still present
    # after the wipe. If a preserved volume was wiped by mistake (e.g. a
    # rogue `docker compose down -v` elsewhere), abort loudly so the
    # operator does not silently lose the Wikidata index or trace history.
    local missing=()
    for v in "${PRESERVED_VOLUMES[@]}"; do
        if ! docker volume inspect "$v" >/dev/null 2>&1; then
            missing+=("$v")
        fi
    done
    if (( ${#missing[@]} > 0 )); then
        echo -e "${RED}✗ preserved volume missing: ${missing[*]}${RESET}"
        echo -e "${GRAY}  Build-time artefacts were expected to survive the wipe; investigate before continuing.${RESET}"
        exit 2
    fi
    echo -e "${GREEN}✔ preserved volumes intact: ${PRESERVED_VOLUMES[*]}${RESET}"
}

case "$TARGET" in
    all)
        confirm_deletion "ALL runtime state (Postgres, MinIO, ClickHouse, RSS crawler dedup)"
        echo -e "${CYAN}ℹ Stopping stack...${RESET}"
        docker compose down --remove-orphans >/dev/null
        echo -e "${CYAN}ℹ Wiping runtime state volumes...${RESET}"
        for v in "${RUNTIME_STATE_VOLUMES[@]}"; do
            remove_volume "$v"
        done
        assert_preserved
        echo -e "${GREEN}✔ Runtime state wiped. Build-time artefacts preserved.${RESET}"
        ;;

    postgres)
        confirm_deletion "PostgreSQL"
        docker compose stop postgres >/dev/null
        docker compose rm -f postgres >/dev/null
        remove_volume "${PROJECT_NAME}_postgres_data"
        # Crawler dedup state is logically tied to Postgres documents;
        # an orphaned state here makes the next crawl skip everything.
        remove_volume "${PROJECT_NAME}_rss_crawler_state"
        ;;

    minio)
        confirm_deletion "MinIO (Bronze + Silver + Quarantine)"
        docker compose stop minio minio-init >/dev/null
        docker compose rm -f minio minio-init >/dev/null
        remove_volume "${PROJECT_NAME}_minio_data"
        # Same coupling as the Postgres path — Bronze and dedup state move together.
        remove_volume "${PROJECT_NAME}_rss_crawler_state"
        ;;

    clickhouse)
        confirm_deletion "ClickHouse (Gold + Silver projection)"
        docker compose stop clickhouse clickhouse-init >/dev/null
        docker compose rm -f clickhouse clickhouse-init >/dev/null
        remove_volume "${PROJECT_NAME}_clickhouse_data"
        ;;

    *)
        echo "Usage: $0 {all|postgres|minio|clickhouse}" >&2
        echo "" >&2
        echo "Set AER_RESET_NONINTERACTIVE=1 to skip the confirmation prompt." >&2
        exit 1
        ;;
esac
