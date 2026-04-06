#!/bin/bash
# End-to-End Smoke Test for the AĒR pipeline.
# Flow: docker compose up → fixture server → RSS crawler → wait → assert all endpoints → teardown
set -euo pipefail

# --- Colors ---
GREEN='\033[38;5;76m'
RED='\033[38;5;196m'
CYAN='\033[38;5;39m'
GOLD='\033[38;5;214m'
RESET='\033[0m'

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

# --- Helpers ---
log_info()  { echo -e "${CYAN}[INFO]${RESET}  $*"; }
log_ok()    { echo -e "${GREEN}[PASS]${RESET}  $*"; PASS=$((PASS + 1)); }
log_fail()  { echo -e "${RED}[FAIL]${RESET}  $*"; FAIL=$((FAIL + 1)); }
log_step()  { echo -e "\n${GOLD}══ $* ${RESET}"; }

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
    "${BFF_URL}/metrics/available" 2>/dev/null) || AVAILABLE_STATUS="000"

if [[ "$AVAILABLE_STATUS" == "200" ]]; then
    # Check that expected metric names are present
    EXPECTED_METRICS=("word_count" "sentiment_score" "entity_count" "language_confidence" "publication_hour" "publication_weekday")
    MISSING=""
    for m in "${EXPECTED_METRICS[@]}"; do
        HAS=$(python3 -c "
import json
d = json.load(open('/tmp/aer_e2e_available.json'))
print('yes' if '$m' in d else 'no')
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
else
    log_fail "metrics/available endpoint returned HTTP $AVAILABLE_STATUS."
    log_info "Response: $(cat /tmp/aer_e2e_available.json 2>/dev/null || echo '<empty>')"
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
