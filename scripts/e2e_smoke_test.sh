#!/bin/bash
# End-to-End Smoke Test for the AĒR pipeline.
# Flow: docker compose up → fixture server → RSS crawler → wait → assert all endpoints → teardown
set -euo pipefail

# --- Load shared helpers (colors, log_info, log_ok, log_fail, log_step) ---
source "$(dirname "$0")/e2e_helpers.sh"

# --- Load .env (provides BFF_API_KEY and other secrets) ---
if [[ -f .env ]]; then
    set -a; source .env; set +a
fi

# --- Config ---
BFF_URL="http://localhost:8080/api/v1"
BFF_API_KEY="${BFF_API_KEY:-}"
INGESTION_API_KEY="${INGESTION_API_KEY:-}"
PROCESSING_WAIT=25    # seconds to give NATS + worker time to process
FIXTURE_DIR="$(cd "$(dirname "$0")/e2e_fixtures" && pwd)"
FIXTURE_CONTAINER="e2e-fixture-server"

PASS=0
FAIL=0

# --- Teardown (always runs) ---
cleanup() {
    local exit_code=$?

    log_step "Teardown"

    # Stop fixture server
    docker rm -f "$FIXTURE_CONTAINER" 2>/dev/null || true

    echo -e "${GOLD}══ Result ══════════════════════════════════${RESET}"
    echo -e "   ${GREEN}PASSED: $PASS${RESET}   ${RED}FAILED: $FAIL${RESET}"

    if [[ $FAIL -gt 0 || $exit_code -ne 0 ]]; then
        LOG_DIR="logs/e2e"
        mkdir -p "$LOG_DIR"
        TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")
        LOG_FILE="${LOG_DIR}/e2e_fail_${TIMESTAMP}.log"

        echo -e "   ${RED}Smoke test FAILED. Dumping full stack logs to ${LOG_FILE}...${RESET}"
        docker compose logs --timestamps --no-color | sort > "$LOG_FILE" 2>&1

        FINAL_EXIT=1
    else
        echo -e "   ${GREEN}Smoke test PASSED.${RESET}"
        FINAL_EXIT=0
    fi

    log_info "Running docker compose down -v ..."
    docker compose down -v --remove-orphans 2>/dev/null || true

    exit $FINAL_EXIT
}
trap cleanup EXIT

# ── Step 1: Start full stack and wait for health ─────────────────────────
log_step "Step 1: Starting full Docker Compose stack (waiting for health)"
docker compose up --build --wait -d
log_ok "Stack started. All services are healthy!"

# ── Step 2: Start fixture HTTP server ────────────────────────────────────
log_step "Step 2: Starting fixture HTTP server on Docker network"

docker run -d \
    --name "$FIXTURE_CONTAINER" \
    --network aer-backend \
    -v "$FIXTURE_DIR":/srv:ro \
    -w /srv \
    python:3.14-slim \
    python -m http.server 8888

# Wait for fixture server to start
sleep 2
log_ok "Fixture server running at http://${FIXTURE_CONTAINER}:8888/"

# ── Step 3: Run RSS crawler against the fixture ─────────────────────────
log_step "Step 3: Running RSS crawler against test fixture"

# Build the RSS crawler
(cd crawlers/rss-crawler && CGO_ENABLED=0 GOOS=linux go build -o ../../bin/rss-crawler . 2>&1) || {
    log_fail "Failed to build RSS crawler"
    exit 1
}

# The crawler runs on the host but submits to ingestion-api inside Docker.
# We use docker compose exec to POST via the ingestion-api container (wget).
# Instead, we run the crawler binary and point it at the fixture server
# accessible via the Docker network. The ingestion API is on localhost:8081
# only in debug mode. So we exec the crawler inside a container.

# Copy the crawler binary and config into a temporary container on the network
docker run -d \
    --name e2e-crawler-runner \
    --network aer-backend \
    -v "$FIXTURE_DIR/feeds.yaml":/feeds.yaml:ro \
    -v "$(pwd)/bin/rss-crawler":/rss-crawler:ro \
    alpine:3.23.3 \
    sleep 300

# Run the crawler inside the Docker network
CRAWLER_OUTPUT=$(docker exec e2e-crawler-runner \
    /rss-crawler \
    --config /feeds.yaml \
    --api-url "http://ingestion-api:8081/api/v1/ingest" \
    --sources-url "http://ingestion-api:8081/api/v1/sources" \
    --api-key "${INGESTION_API_KEY}" \
    --state /tmp/e2e-state.json \
    --delay 0 2>&1) && CRAWLER_OK=true || CRAWLER_OK=false

# Clean up crawler runner
docker rm -f e2e-crawler-runner 2>/dev/null || true

if $CRAWLER_OK; then
    log_ok "RSS crawler completed successfully."
    log_info "Crawler output: $CRAWLER_OUTPUT"
else
    log_fail "RSS crawler failed."
    log_info "Crawler output: ${CRAWLER_OUTPUT:-<empty>}"
fi

# ── Step 4: Wait for pipeline processing ─────────────────────────────────
log_step "Step 4: Waiting ${PROCESSING_WAIT}s for NATS + worker processing"
sleep "$PROCESSING_WAIT"

# ── Step 5: Assert GET /api/v1/metrics?metricName=word_count ─────────────
log_step "Step 5: GET /api/v1/metrics?metricName=word_count"

NOW=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
ONE_HOUR_AGO=$(date -u -d "1 hour ago" +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null \
    || date -u -v-1H +"%Y-%m-%dT%H:%M:%SZ")

METRICS_STATUS=$(curl -sf \
    -o /tmp/aer_e2e_wc.json \
    -w "%{http_code}" \
    -H "X-API-Key: ${BFF_API_KEY}" \
    "${BFF_URL}/metrics?metricName=word_count&startDate=${ONE_HOUR_AGO}&endDate=${NOW}" 2>/dev/null) || METRICS_STATUS="000"

if [[ "$METRICS_STATUS" == "200" ]]; then
    WC_COUNT=$(python3 -c "import json; d=json.load(open('/tmp/aer_e2e_wc.json')); print(len(d) if isinstance(d,list) else 0)" 2>/dev/null || echo "0")
    if [[ "$WC_COUNT" -gt 0 ]]; then
        log_ok "word_count metrics: $WC_COUNT data point(s)."
    else
        log_fail "word_count returned 200 but 0 data points."
        log_info "Response: $(cat /tmp/aer_e2e_wc.json)"
    fi
else
    log_fail "word_count endpoint returned HTTP $METRICS_STATUS."
    log_info "Response: $(cat /tmp/aer_e2e_wc.json 2>/dev/null || echo '<empty>')"
fi

# ── Step 6: Assert GET /api/v1/metrics?metricName=sentiment_score ────────
log_step "Step 6: GET /api/v1/metrics?metricName=sentiment_score"

SENTIMENT_STATUS=$(curl -sf \
    -o /tmp/aer_e2e_sent.json \
    -w "%{http_code}" \
    -H "X-API-Key: ${BFF_API_KEY}" \
    "${BFF_URL}/metrics?metricName=sentiment_score&startDate=${ONE_HOUR_AGO}&endDate=${NOW}" 2>/dev/null) || SENTIMENT_STATUS="000"

if [[ "$SENTIMENT_STATUS" == "200" ]]; then
    SENT_COUNT=$(python3 -c "import json; d=json.load(open('/tmp/aer_e2e_sent.json')); print(len(d) if isinstance(d,list) else 0)" 2>/dev/null || echo "0")
    if [[ "$SENT_COUNT" -gt 0 ]]; then
        # Verify values are within expected range [-1, 1]
        SENT_VALID=$(python3 -c "
import json
d = json.load(open('/tmp/aer_e2e_sent.json'))
print('ok' if all(-1.0 <= p['value'] <= 1.0 for p in d) else 'out_of_range')
" 2>/dev/null || echo "error")
        if [[ "$SENT_VALID" == "ok" ]]; then
            log_ok "sentiment_score metrics: $SENT_COUNT data point(s), all within [-1, 1]."
        else
            log_fail "sentiment_score values out of expected range [-1, 1]."
            log_info "Response: $(cat /tmp/aer_e2e_sent.json)"
        fi
    else
        log_fail "sentiment_score returned 200 but 0 data points."
        log_info "Response: $(cat /tmp/aer_e2e_sent.json)"
    fi
else
    log_fail "sentiment_score endpoint returned HTTP $SENTIMENT_STATUS."
fi

# ── Step 7: Assert GET /api/v1/entities ──────────────────────────────────
log_step "Step 7: GET /api/v1/entities"

ENTITIES_STATUS=$(curl -sf \
    -o /tmp/aer_e2e_entities.json \
    -w "%{http_code}" \
    -H "X-API-Key: ${BFF_API_KEY}" \
    "${BFF_URL}/entities?startDate=${ONE_HOUR_AGO}&endDate=${NOW}" 2>/dev/null) || ENTITIES_STATUS="000"

if [[ "$ENTITIES_STATUS" == "200" ]]; then
    ENT_COUNT=$(python3 -c "import json; d=json.load(open('/tmp/aer_e2e_entities.json')); print(len(d) if isinstance(d,list) else 0)" 2>/dev/null || echo "0")
    if [[ "$ENT_COUNT" -gt 0 ]]; then
        log_ok "entities endpoint: $ENT_COUNT distinct entities."
    else
        log_fail "entities returned 200 but 0 results."
        log_info "Response: $(cat /tmp/aer_e2e_entities.json)"
    fi
else
    log_fail "entities endpoint returned HTTP $ENTITIES_STATUS."
    log_info "Response: $(cat /tmp/aer_e2e_entities.json 2>/dev/null || echo '<empty>')"
fi

# ── Step 8: Assert GET /api/v1/metrics/available ─────────────────────────
log_step "Step 8: GET /api/v1/metrics/available"

AVAILABLE_STATUS=$(curl -sf \
    -o /tmp/aer_e2e_available.json \
    -w "%{http_code}" \
    -H "X-API-Key: ${BFF_API_KEY}" \
    "${BFF_URL}/metrics/available?startDate=${ONE_HOUR_AGO}&endDate=${NOW}" 2>/dev/null) || AVAILABLE_STATUS="000"

if [[ "$AVAILABLE_STATUS" == "200" ]]; then
    # Check that expected metric names are present
    EXPECTED_METRICS=("word_count" "sentiment_score" "entity_count" "language_confidence" "publication_hour" "publication_weekday")
    MISSING=""
    for m in "${EXPECTED_METRICS[@]}"; do
        HAS=$(python3 -c "
import json
d = json.load(open('/tmp/aer_e2e_available.json'))
names = {item.get('metricName') for item in d if isinstance(item, dict)}
print('yes' if '$m' in names else 'no')
" 2>/dev/null || echo "error")
        if [[ "$HAS" != "yes" ]]; then
            MISSING="${MISSING} ${m}"
        fi
    done

    if [[ -z "$MISSING" ]]; then
        log_ok "metrics/available lists all expected metric names."
    else
        log_fail "metrics/available is missing:${MISSING}"
        log_info "Response: $(cat /tmp/aer_e2e_available.json)"
    fi

    # Assert that lexicon_version is NOT present (Phase 46: provenance removed from Gold layer)
    HAS_LEXICON_VERSION=$(python3 -c "
import json
d = json.load(open('/tmp/aer_e2e_available.json'))
names = {item.get('metricName') for item in d if isinstance(item, dict)}
print('yes' if 'lexicon_version' in names else 'no')
" 2>/dev/null || echo "error")
    if [[ "$HAS_LEXICON_VERSION" == "no" ]]; then
        log_ok "metrics/available correctly excludes lexicon_version (provenance in Silver envelope)."
    else
        log_fail "metrics/available must not contain lexicon_version — provenance belongs in the Silver envelope, not the Gold metrics table."
    fi
else
    log_fail "metrics/available endpoint returned HTTP $AVAILABLE_STATUS."
    log_info "Response: $(cat /tmp/aer_e2e_available.json 2>/dev/null || echo '<empty>')"
fi

# ── Step 8b: Assert discourse_function is populated in ClickHouse (Phase 62) ─
# The aer_gold.metrics.discourse_function column is written by the analysis
# worker when a source_classifications row exists for the source. It is not
# (yet) surfaced via the BFF API, so we assert directly against ClickHouse.
log_step "Step 8b: Assert discourse_function populated in aer_gold.metrics (Phase 62)"

# Note: metric timestamp comes from the article's publication date, not
# ingest time, so we do not filter by timestamp here — the stack is torn
# down after every run, so any matching row is from the current execution.
set +e
DISCOURSE_OUT=$(docker compose exec -T clickhouse clickhouse-client \
    --user="${CLICKHOUSE_USER}" --password="${CLICKHOUSE_PASSWORD}" \
    --query="SELECT count() FROM aer_gold.metrics WHERE metric_name = 'word_count' AND discourse_function != ''" 2>&1)
DISCOURSE_RC=$?
set -e
DISCOURSE_COUNT=$(echo "$DISCOURSE_OUT" | tr -d '[:space:]')

if [[ $DISCOURSE_RC -ne 0 ]]; then
    log_fail "clickhouse-client failed (rc=$DISCOURSE_RC): ${DISCOURSE_OUT}"
elif [[ "${DISCOURSE_COUNT:-0}" -gt 0 ]]; then
    log_ok "discourse_function populated on $DISCOURSE_COUNT row(s) in aer_gold.metrics."
else
    # Distinguish seed missing from worker propagation failure
    SEED_OUT=$(docker compose exec -T postgres psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -tAc \
        "SELECT count(*) FROM source_classifications sc JOIN sources s ON sc.source_id = s.id WHERE s.name = 'bundesregierung'" 2>&1 | tr -d '[:space:]')
    log_fail "discourse_function empty for all word_count rows. source_classifications rows for 'bundesregierung' in postgres: '${SEED_OUT}'. If 0 → migration 000006 didn't seed; if >0 → worker lookup in RssAdapter.harmonize returned None."
fi

# ── Step 8c: Assert GET /api/v1/metrics?resolution=hourly (Phase 66) ──────
log_step "Step 8c: GET /api/v1/metrics?resolution=hourly (Phase 66)"

HOURLY_STATUS=$(curl -sf \
    -o /tmp/aer_e2e_hourly.json \
    -w "%{http_code}" \
    -H "X-API-Key: ${BFF_API_KEY}" \
    "${BFF_URL}/metrics?resolution=hourly&metricName=word_count&startDate=${ONE_HOUR_AGO}&endDate=${NOW}" 2>/dev/null) || HOURLY_STATUS="000"

if [[ "$HOURLY_STATUS" == "200" ]]; then
    HOURLY_COUNT=$(python3 -c "import json; d=json.load(open('/tmp/aer_e2e_hourly.json')); print(len(d) if isinstance(d,list) else 0)" 2>/dev/null || echo "0")
    if [[ "$HOURLY_COUNT" -gt 0 ]]; then
        log_ok "metrics?resolution=hourly: $HOURLY_COUNT data point(s)."
    else
        log_fail "metrics?resolution=hourly returned 200 but 0 data points."
        log_info "Response: $(cat /tmp/aer_e2e_hourly.json)"
    fi
else
    log_fail "metrics?resolution=hourly returned HTTP $HOURLY_STATUS."
    log_info "Response: $(cat /tmp/aer_e2e_hourly.json 2>/dev/null || echo '<empty>')"
fi

# ── Step 8d: Assert GET /api/v1/metrics/{metricName}/provenance (Phase 67) ─
log_step "Step 8d: GET /api/v1/metrics/word_count/provenance (Phase 67)"

PROVENANCE_STATUS=$(curl -sf \
    -o /tmp/aer_e2e_provenance.json \
    -w "%{http_code}" \
    -H "X-API-Key: ${BFF_API_KEY}" \
    "${BFF_URL}/metrics/word_count/provenance" 2>/dev/null) || PROVENANCE_STATUS="000"

if [[ "$PROVENANCE_STATUS" == "200" ]]; then
    PROVENANCE_OK=$(python3 -c "
import json
d = json.load(open('/tmp/aer_e2e_provenance.json'))
required = ('tierClassification', 'algorithmDescription', 'knownLimitations', 'validationStatus', 'metricName')
missing = [f for f in required if f not in d]
if missing:
    print('missing:' + ','.join(missing))
elif not d['algorithmDescription']:
    print('empty_algorithm_description')
elif d['tierClassification'] not in (1, 2, 3):
    print('invalid_tier:' + str(d['tierClassification']))
else:
    print('ok')
" 2>/dev/null || echo "error")
    if [[ "$PROVENANCE_OK" == "ok" ]]; then
        log_ok "provenance endpoint returns tier + algorithm description for word_count."
    else
        log_fail "provenance endpoint response invalid: $PROVENANCE_OK"
        log_info "Response: $(cat /tmp/aer_e2e_provenance.json)"
    fi
else
    log_fail "provenance endpoint returned HTTP $PROVENANCE_STATUS."
    log_info "Response: $(cat /tmp/aer_e2e_provenance.json 2>/dev/null || echo '<empty>')"
fi

# ── Step 9: Assert GET /api/v1/languages ────────────────────────────────
log_step "Step 9: GET /api/v1/languages"

LANG_STATUS=$(curl -sf \
    -o /tmp/aer_e2e_lang.json \
    -w "%{http_code}" \
    -H "X-API-Key: ${BFF_API_KEY}" \
    "${BFF_URL}/languages?startDate=${ONE_HOUR_AGO}&endDate=${NOW}" 2>/dev/null) || LANG_STATUS="000"

if [[ "$LANG_STATUS" == "200" ]]; then
    LANG_COUNT=$(python3 -c "import json; d=json.load(open('/tmp/aer_e2e_lang.json')); print(len(d) if isinstance(d,list) else 0)" 2>/dev/null || echo "0")
    if [[ "$LANG_COUNT" -gt 0 ]]; then
        # Verify at least one entry has detected_language = "de"
        HAS_DE=$(python3 -c "
import json
d = json.load(open('/tmp/aer_e2e_lang.json'))
print('yes' if any(e.get('detectedLanguage') == 'de' for e in d) else 'no')
" 2>/dev/null || echo "error")
        if [[ "$HAS_DE" == "yes" ]]; then
            log_ok "languages endpoint: $LANG_COUNT language(s), includes 'de'."
        else
            log_fail "languages returned $LANG_COUNT entries but none with detectedLanguage=de."
            log_info "Response: $(cat /tmp/aer_e2e_lang.json)"
        fi
    else
        log_fail "languages returned 200 but 0 results."
        log_info "Response: $(cat /tmp/aer_e2e_lang.json)"
    fi
else
    log_fail "languages endpoint returned HTTP $LANG_STATUS."
    log_info "Response: $(cat /tmp/aer_e2e_lang.json 2>/dev/null || echo '<empty>')"
fi
