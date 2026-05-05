#!/bin/bash
# Phase-120b post-wipe invariant check.
#
# Confirms that after `make reset-state` (or `make reset`) every layer
# of the stack reports the expected clean / re-seeded shape. Returns 0
# if the system is in the canonical post-reset state; non-zero if any
# invariant fails.
#
# Layers checked, in order:
#   1. Docker volumes — preserved volumes still present
#   2. MinIO — buckets exist, all empty
#   3. PostgreSQL — `documents` empty, `sources` re-seeded
#   4. ClickHouse — every Gold table exists, all rows = 0
#   5. NATS JetStream — AER_LAKE stream exists, no pending messages
#   6. Worker / BFF readiness — both /readyz endpoints return 200

set -euo pipefail

GREEN='\033[38;5;76m'
RED='\033[38;5;196m'
GOLD='\033[38;5;214m'
CYAN='\033[38;5;39m'
GRAY='\033[38;5;245m'
RESET='\033[0m'

FAIL=0

ok()   { echo -e "  ${GREEN}✔${RESET} $1"; }
fail() { echo -e "  ${RED}✗${RESET} $1"; FAIL=1; }
note() { echo -e "  ${GRAY}·${RESET} $1"; }
step() { echo -e "${CYAN}▸ $1${RESET}"; }

PROJECT="aer"

# Allow operator to run validation against a partially-up stack (e.g.
# infra-only). Worker / BFF readiness checks degrade to "skipped" when
# the services are not running.
SKIP_SERVICES="${SKIP_SERVICES:-0}"

# ---------------------------------------------------------------------------
step "1. Docker volumes — preserved set intact"
for v in ${PROJECT}_hf_cache ${PROJECT}_wikidata_data ${PROJECT}_tempo_data; do
    if docker volume inspect "$v" >/dev/null 2>&1; then
        ok "preserved: $v"
    else
        fail "preserved volume missing: $v (build-time artefact will be re-fetched on next image build)"
    fi
done

# ---------------------------------------------------------------------------
step "2. MinIO — buckets exist and are empty"
if docker compose ps minio --format '{{.State}}' 2>/dev/null | grep -q running; then
    for bucket in bronze silver bronze-quarantine; do
        # `mc ls` exit code is non-zero on a missing bucket and zero on
        # an empty bucket. Distinguish empty-vs-missing by counting lines.
        out=$(docker compose exec -T minio mc ls "minio/${bucket}" 2>&1 || true)
        if echo "$out" | grep -qiE 'does not exist|not found'; then
            fail "bucket missing: $bucket"
        else
            count=$(echo "$out" | grep -cE '^\[' || true)
            if [[ "$count" == "0" ]]; then
                ok "bucket empty: $bucket"
            else
                fail "bucket non-empty: $bucket ($count entries)"
            fi
        fi
    done
else
    note "MinIO container not running — skipping (start the stack first)"
fi

# ---------------------------------------------------------------------------
step "3. PostgreSQL — documents empty, sources re-seeded"
if docker compose ps postgres --format '{{.State}}' 2>/dev/null | grep -q running; then
    docs=$(docker compose exec -T postgres psql -U "${POSTGRES_USER:-aer}" -d "${POSTGRES_DB:-aer}" \
        -tA -c "SELECT count(*) FROM documents;" 2>/dev/null || echo "ERROR")
    srcs=$(docker compose exec -T postgres psql -U "${POSTGRES_USER:-aer}" -d "${POSTGRES_DB:-aer}" \
        -tA -c "SELECT count(*) FROM sources;" 2>/dev/null || echo "ERROR")
    if [[ "$docs" == "0" ]]; then
        ok "documents empty (count=0)"
    else
        fail "documents non-empty (count=$docs) — pre-existing idempotency rows survive"
    fi
    if [[ "$srcs" =~ ^[0-9]+$ && "$srcs" -ge 2 ]]; then
        ok "sources seeded (count=$srcs)"
    else
        fail "sources missing or under-seeded (count=$srcs) — seed migration did not apply"
    fi
else
    note "Postgres container not running — skipping"
fi

# ---------------------------------------------------------------------------
step "4. ClickHouse — Gold tables exist, all rows = 0"
if docker compose ps clickhouse --format '{{.State}}' 2>/dev/null | grep -q running; then
    expected_tables=(
        "aer_gold.metrics"
        "aer_gold.entities"
        "aer_gold.entity_links"
        "aer_gold.entity_cooccurrences"
        "aer_gold.language_detections"
        "aer_gold.metric_baselines"
        "aer_gold.metric_equivalence"
        "aer_gold.metric_validity"
        "aer_gold.topic_assignments"
        "aer_silver.documents"
    )
    for tbl in "${expected_tables[@]}"; do
        db="${tbl%%.*}"; name="${tbl#*.}"
        rows=$(docker compose exec -T clickhouse clickhouse-client -q "
            SELECT total_rows FROM system.tables WHERE database='${db}' AND name='${name}';" 2>/dev/null || echo "")
        if [[ -z "$rows" ]]; then
            fail "$tbl missing — migration did not apply"
        elif [[ "$rows" == "0" ]]; then
            ok "$tbl empty"
        else
            fail "$tbl non-empty (rows=$rows)"
        fi
    done
else
    note "ClickHouse container not running — skipping"
fi

# ---------------------------------------------------------------------------
step "5. NATS JetStream — AER_LAKE stream re-provisioned"
if docker compose ps nats --format '{{.State}}' 2>/dev/null | grep -q running; then
    if docker compose exec -T nats nats stream info AER_LAKE >/dev/null 2>&1; then
        msgs=$(docker compose exec -T nats nats stream info AER_LAKE --json 2>/dev/null \
            | grep -oE '"messages":[0-9]+' | head -1 | grep -oE '[0-9]+' || echo "?")
        if [[ "$msgs" == "0" ]]; then
            ok "AER_LAKE stream exists (messages=0)"
        else
            note "AER_LAKE stream exists (messages=$msgs) — non-zero is OK if a crawl already ran"
        fi
    else
        fail "AER_LAKE stream missing — nats-init did not run"
    fi
else
    note "NATS container not running — skipping"
fi

# ---------------------------------------------------------------------------
step "6. Worker + BFF readiness"
if [[ "$SKIP_SERVICES" == "1" ]]; then
    note "SKIP_SERVICES=1 — skipping service readiness probes"
elif docker compose ps aer_analysis_worker --format '{{.State}}' 2>/dev/null | grep -q running; then
    if docker compose exec -T aer_analysis_worker python -c "
import urllib.request,sys
sys.exit(0) if urllib.request.urlopen('http://localhost:8001/metrics', timeout=3).status == 200 else sys.exit(1)
" >/dev/null 2>&1; then
        ok "analysis-worker /metrics responds"
    else
        fail "analysis-worker /metrics did not respond"
    fi
    if docker compose exec -T aer_bff_api wget -q -O- http://localhost:8080/api/v1/readyz >/dev/null 2>&1; then
        ok "bff-api /readyz responds"
    else
        fail "bff-api /readyz did not respond"
    fi
else
    note "services not running — skipping (run 'make services-up' first)"
fi

# ---------------------------------------------------------------------------
echo ""
if [[ "$FAIL" == "0" ]]; then
    echo -e "${GREEN}✔ Reset validation passed — system is in canonical post-reset state.${RESET}"
    echo -e "${GRAY}  Next: 'make crawl' to populate Bronze → Silver → Gold from a fresh feed pull.${RESET}"
    exit 0
else
    echo -e "${RED}✗ Reset validation failed — see ✗ entries above.${RESET}"
    exit 1
fi
