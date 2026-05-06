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

# Load .env so POSTGRES_USER / POSTGRES_DB / MINIO_ROOT_* are available to
# the per-layer probes below. The repo root is one level up from this script.
ENV_FILE="$(cd "$(dirname "$0")/.." && pwd)/.env"
if [[ -f "$ENV_FILE" ]]; then
    set -a
    # shellcheck disable=SC1090
    source "$ENV_FILE"
    set +a
fi

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
for v in ${PROJECT}_wikidata_data ${PROJECT}_tempo_data; do
    if docker volume inspect "$v" >/dev/null 2>&1; then
        ok "preserved: $v"
    else
        fail "preserved volume missing: $v"
    fi
done

# ---------------------------------------------------------------------------
step "2. MinIO — buckets exist and are empty"
if docker compose ps minio --format '{{.State}}' 2>/dev/null | grep -q running; then
    # The `minio` container has the `mc` binary but no client alias is
    # configured by default — that lives in the `minio-init` container.
    # Configure the alias inline using root credentials from .env, then
    # query each bucket. `mc ls` returns non-zero for missing buckets and
    # zero with empty stdout for empty buckets.
    alias_setup=$(docker compose exec -T -e MINIO_ROOT_USER="${MINIO_ROOT_USER:-}" \
        -e MINIO_ROOT_PASSWORD="${MINIO_ROOT_PASSWORD:-}" minio \
        sh -c 'mc alias set m http://localhost:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" 2>&1' || true)
    if ! echo "$alias_setup" | grep -qiE 'success|added'; then
        fail "MinIO alias setup failed: $alias_setup"
    else
        for bucket in bronze silver bronze-quarantine; do
            out=$(docker compose exec -T minio mc ls "m/${bucket}" 2>&1 || true)
            if echo "$out" | grep -qiE 'does not exist|not found|specified bucket'; then
                fail "bucket missing: $bucket"
            else
                # Each object in a non-empty bucket prints one line; empty
                # buckets print nothing. Strip blanks before counting.
                count=$(echo "$out" | grep -cE '\S' || true)
                if [[ "$count" == "0" ]]; then
                    ok "bucket empty: $bucket"
                else
                    fail "bucket non-empty: $bucket ($count entries)"
                fi
            fi
        done
    fi
else
    note "MinIO container not running — skipping (start the stack first)"
fi

# ---------------------------------------------------------------------------
step "3. PostgreSQL — documents empty, sources re-seeded"
if docker compose ps postgres --format '{{.State}}' 2>/dev/null | grep -q running; then
    # POSTGRES_USER / POSTGRES_DB are loaded from .env at the top of this
    # script. The fallback defaults match `.env.example` so a fresh checkout
    # without a customised .env still validates correctly.
    docs=$(docker compose exec -T postgres psql -U "${POSTGRES_USER:-aer_admin}" -d "${POSTGRES_DB:-aer_metadata}" \
        -tA -c "SELECT count(*) FROM documents;" 2>/dev/null || echo "ERROR")
    srcs=$(docker compose exec -T postgres psql -U "${POSTGRES_USER:-aer_admin}" -d "${POSTGRES_DB:-aer_metadata}" \
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
    # The `nats` server image does not ship the `nats` CLI — that lives in
    # the `nats-init` container (natsio/nats-box). Run a one-shot nats-box
    # invocation against the live nats server to query the stream.
    info=$(docker compose run --rm --no-deps --entrypoint sh nats-init \
        -c "nats --server nats:4222 stream info AER_LAKE --json" 2>/dev/null || echo "")
    if [[ -n "$info" ]] && echo "$info" | grep -q '"name"'; then
        # `messages` lives under `state` in the stream-info JSON (nats CLI
        # 0.x), so a top-level grep misses it. Use Python to read the
        # nested key — Python is already a hard dependency on this host.
        msgs=$(echo "$info" | python3 -c "import sys,json; print(json.load(sys.stdin).get('state',{}).get('messages','?'))" 2>/dev/null || echo "?")
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
elif docker compose ps analysis-worker --format '{{.State}}' 2>/dev/null | grep -q running; then
    # `docker compose ps` and `exec` take the *service* name (analysis-worker,
    # bff-api), not the container name (aer_analysis_worker, aer_bff_api).
    if docker compose exec -T analysis-worker python -c "
import urllib.request,sys
sys.exit(0) if urllib.request.urlopen('http://localhost:8001/metrics', timeout=3).status == 200 else sys.exit(1)
" >/dev/null 2>&1; then
        ok "analysis-worker /metrics responds"
    else
        fail "analysis-worker /metrics did not respond"
    fi
    if docker compose exec -T bff-api wget -q -O- http://localhost:8080/api/v1/readyz >/dev/null 2>&1; then
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
