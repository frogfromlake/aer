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
#   4. ClickHouse — analytical Gold tables empty; seeded equivalence grants present
#   5. NATS JetStream — AER_LAKE stream exists, no pending messages
#   6. Worker / BFF readiness — both /readyz endpoints return 200

set -euo pipefail

# Load .env so POSTGRES_USER / POSTGRES_DB / MINIO_ROOT_* are available to
# the per-layer probes below. This script lives at
# `scripts/operations/reset_validate.sh`, so the repo root is **two**
# directory levels up — not one. The previous `dirname/..` form silently
# resolved to `scripts/.env` (which doesn't exist), the source step was
# a no-op, and downstream probes ran with empty MINIO_ROOT_* / etc.,
# producing misleading `AccessDenied` failures from `mc du`.
#
# `.env` is consumed by Docker Compose with FLAT-FILE semantics: each
# line is a literal `KEY=VALUE`, the value runs verbatim to end-of-line,
# and shell metacharacters (parens, quotes, semicolons) are NOT
# interpreted. AĒR's `WEB_CRAWLER_USER_AGENT` value carries unescaped
# parens and a semicolon to satisfy the WP-006 §5.1 contact-address
# convention. `source .env` would invoke shell parsing on those values
# and crash with a syntax error. Use a per-line read loop that exports
# only well-formed identifiers and treats the rest of the line as
# verbatim payload — same semantics Compose applies.
ENV_FILE="$(cd "$(dirname "$0")/../.." && pwd)/.env"
if [[ -f "$ENV_FILE" ]]; then
    # Read whole lines (no IFS split). Splitting on `=` with `read -r KEY
    # VALUE` strips a *trailing* `=` from the value because POSIX `read`
    # trims trailing IFS characters from the last field — and AĒR's
    # secrets (MINIO_ROOT_PASSWORD, BFF_API_KEY, INGESTION_API_KEY) are
    # base64-encoded and end in `=`. The truncation produced wrong-but-
    # plausible credentials and caused mc's "request signature does not
    # match" error in step 2 of this validator. Capture the whole line
    # and split on the first `=` via regex — `(.*)` preserves every
    # trailing character including additional `=`.
    while IFS= read -r _line || [[ -n "$_line" ]]; do
        # Skip blank lines and comments.
        [[ -z "${_line// }" ]] && continue
        [[ "$_line" =~ ^[[:space:]]*# ]] && continue
        if [[ "$_line" =~ ^([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]]; then
            export "${BASH_REMATCH[1]}=${BASH_REMATCH[2]}"
        fi
    done < "$ENV_FILE"
    unset _line
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
    # Important detail learned from the Phase-122e first-crawl session:
    # **the mc alias is set per-exec-session and does NOT persist across
    # separate `docker compose exec` calls.** A previous implementation
    # ran `mc alias set m ...` in one exec and `mc ls m/<bucket>` in
    # subsequent execs, which produced a silent `AccessDenied` error
    # (alias `m` was unset in the new session and mc fell through to an
    # anonymous query) — and the legacy line-counting empty-check
    # mis-classified the access-denied error message as a single bucket
    # entry, producing the misleading `bucket non-empty: <name> (1 entries)`
    # output. Fix: do the alias setup AND all bucket queries in one
    # `docker compose exec` call so they share the same mc-config state.
    #
    # Empty-check uses `mc du --json` — the only mc command that
    # definitively counts objects (not virtual prefixes, ILM-policy
    # artefacts, or other non-data surface entries). Returns
    # `"objects":0` for empty buckets regardless of attached lifecycle
    # policies. Plain-text grep avoids a `jq` dependency.
    out=$(docker compose exec -T -e MINIO_ROOT_USER="${MINIO_ROOT_USER:-}" \
        -e MINIO_ROOT_PASSWORD="${MINIO_ROOT_PASSWORD:-}" minio sh -c '
            mc alias set m http://localhost:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" 2>&1 | tail -1
            for b in bronze silver bronze-quarantine; do
                printf "::bucket::%s::" "$b"
                mc du --json "m/$b" 2>&1
                printf "\n"
            done
        ' 2>&1 || true)
    if ! echo "$out" | grep -qiE 'success|added'; then
        fail "MinIO alias setup failed: $out"
    else
        for bucket in bronze silver bronze-quarantine; do
            line=$(echo "$out" | grep -F "::bucket::${bucket}::" | head -1)
            if [[ -z "$line" ]]; then
                fail "bucket query produced no output: $bucket"
                continue
            fi
            payload="${line#*::bucket::${bucket}::}"
            if echo "$payload" | grep -qiE 'does not exist|not found|specified bucket'; then
                fail "bucket missing: $bucket"
            elif echo "$payload" | grep -qiE 'access denied|signature|unauthorised|unauthorized'; then
                fail "bucket query auth failed: $bucket — $payload"
            else
                objects=$(echo "$payload" | grep -oE '"objects":[0-9]+' | head -1 | cut -d: -f2)
                if [[ -z "$objects" ]]; then
                    fail "bucket query produced no object count: $bucket — $payload"
                elif [[ "$objects" == "0" ]]; then
                    ok "bucket empty: $bucket"
                else
                    fail "bucket non-empty: $bucket ($objects objects)"
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
step "4. ClickHouse — analytical Gold tables empty, seeded grants present"
if docker compose ps clickhouse --format '{{.State}}' 2>/dev/null | grep -q running; then
    # These tables hold per-article ANALYTICAL output and must be empty
    # after a reset. NOTE: `aer_gold.metric_equivalence` is NOT here — its
    # cross-cultural equivalence grants are SEED data applied by migration
    # 000028 (Phase 124), so it is expected to be re-seeded, not empty
    # (validated separately below, like Postgres `sources`).
    expected_tables=(
        "aer_gold.metrics"
        "aer_gold.entities"
        "aer_gold.entity_links"
        "aer_gold.entity_cooccurrences"
        "aer_gold.language_detections"
        "aer_gold.metric_baselines"
        "aer_gold.metric_validity"
        "aer_gold.topic_assignments"
        "aer_gold.article_metadata"
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

    # metric_equivalence: seeded grants must be re-applied (migration 000028
    # seeds the temporal Level-1 grants for publication_hour/publication_weekday
    # × de/fr). Use FINAL to collapse the ReplacingMergeTree before counting.
    grants=$(docker compose exec -T clickhouse clickhouse-client -q "
        SELECT count() FROM aer_gold.metric_equivalence FINAL;" 2>/dev/null || echo "ERROR")
    if [[ "$grants" =~ ^[0-9]+$ && "$grants" -ge 1 ]]; then
        ok "aer_gold.metric_equivalence seeded (grants=$grants)"
    else
        fail "aer_gold.metric_equivalence not seeded (count=$grants) — seed migration did not apply"
    fi

    # SEC-057 — lifecycle structures read-back. The three AggregatingMergeTree
    # resolution MVs carry the long-term retention (WP-005 §5.4: hourly 365d,
    # daily 1825d, monthly indefinite) and the raw `metrics` TTL bounds disk on
    # the article published_date. If a migration silently no-ops, retention +
    # disk-bounding break invisibly — so assert the structures exist here.
    for mv in metrics_hourly metrics_daily metrics_monthly; do
        present=$(docker compose exec -T clickhouse clickhouse-client -q "
            SELECT count() FROM system.tables WHERE database='aer_gold' AND name='${mv}';" 2>/dev/null || echo "")
        if [[ "$present" == "1" ]]; then
            ok "resolution MV present: aer_gold.${mv}"
        else
            fail "resolution MV missing: aer_gold.${mv} — Phase 122c migration 000019 did not apply"
        fi
    done
    ttl=$(docker compose exec -T clickhouse clickhouse-client -q "
        SHOW CREATE TABLE aer_gold.metrics;" 2>/dev/null | grep -c "TTL" || true)
    if [[ "$ttl" -ge 1 ]]; then
        ok "aer_gold.metrics carries a TTL (disk-bounding ILM present)"
    else
        fail "aer_gold.metrics has NO TTL — the 365d retention did not apply"
    fi
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
