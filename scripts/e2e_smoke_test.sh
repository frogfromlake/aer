#!/bin/bash
# End-to-End Smoke Test for the AĒR pipeline.
# Flow: docker compose up --wait → ingest → wait → metrics → docker compose down
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
PROCESSING_WAIT=15    # seconds to give NATS + worker time to process

PASS=0
FAIL=0

# --- Helpers ---
log_info()  { echo -e "${CYAN}[INFO]${RESET}  $*"; }
log_ok()    { echo -e "${GREEN}[PASS]${RESET}  $*"; PASS=$((PASS + 1)); }
log_fail()  { echo -e "${RED}[FAIL]${RESET}  $*"; FAIL=$((FAIL + 1)); }
log_step()  { echo -e "\n${GOLD}══ $* ${RESET}"; }

# --- Teardown (always runs) ---
cleanup() {
    # Save exit code of the last command in case of unexpected script crash
    local exit_code=$?

    log_step "Teardown"
    echo -e "${GOLD}══ Result ══════════════════════════════════${RESET}"
    echo -e "   ${GREEN}PASSED: $PASS${RESET}   ${RED}FAILED: $FAIL${RESET}"

    # If FAIL > 0, or the script crashed hard (exit_code != 0)
    if [[ $FAIL -gt 0 || $exit_code -ne 0 ]]; then
        LOG_DIR="logs/e2e"
        mkdir -p "$LOG_DIR"
        TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")
        LOG_FILE="${LOG_DIR}/e2e_fail_${TIMESTAMP}.log"

        echo -e "   ${RED}Smoke test FAILED. Dumping full stack logs to ${LOG_FILE}...${RESET}"
        # Dump ALL logs to the file
        docker compose logs --timestamps --no-color | sort > "$LOG_FILE" 2>&1

        FINAL_EXIT=1
    else
        echo -e "   ${GREEN}Smoke test PASSED.${RESET}"
        FINAL_EXIT=0
    fi

    # Unconditional teardown! This runs always, regardless of success or failure.
    log_info "Running docker compose down -v ..."
    docker compose down -v --remove-orphans 2>/dev/null || true

    exit $FINAL_EXIT
}
trap cleanup EXIT

# ── Step 1: Start full stack and wait for health ─────────────────────────
log_step "Step 1: Starting full Docker Compose stack (waiting for health)"
docker compose up --build --wait -d
log_ok "Stack started. All services are healthy!"

# ── Step 2: Ingest a test document ───────────────────────────────────────────
log_step "Step 2: POST test document to Ingestion API"

PAYLOAD='{
  "source_id": 1,
  "documents": [
    {
      "key": "e2e-smoke-test-doc-001",
      "data": {
        "title": "E2E Smoke Test Article",
        "raw_text": "This is an automated end-to-end smoke test document for the AER pipeline. It validates the full flow from ingestion through the NATS broker, the analysis worker, ClickHouse, and finally the BFF API.",
        "url": "https://example.com/smoke-test",
        "source": "e2e-smoke-test"
      }
    }
  ]
}'

# Ingestion API has no host port — exec into the container and use wget (Alpine).
INGEST_RESPONSE=$(docker compose exec -T ingestion-api \
    wget -q -O - \
    --header="Content-Type: application/json" \
    --post-data="$PAYLOAD" \
    "http://localhost:8081/api/v1/ingest" 2>/dev/null) && INGEST_OK=true || INGEST_OK=false

if $INGEST_OK; then
    log_ok "Ingestion returned HTTP 2xx."
    log_info "Response: $INGEST_RESPONSE"
else
    log_fail "Ingestion request failed."
    log_info "Response body: ${INGEST_RESPONSE:-<empty>}"
fi

# ── Step 3: Wait for pipeline processing ─────────────────────────────────────
log_step "Step 3: Waiting ${PROCESSING_WAIT}s for NATS + worker processing"
sleep "$PROCESSING_WAIT"

# ── Step 4: Query BFF metrics ─────────────────────────────────────────────────
log_step "Step 4: GET /api/v1/metrics (last 5 minutes)"

NOW=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
FIVE_MIN_AGO=$(date -u -d "5 minutes ago" +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null \
    || date -u -v-5M +"%Y-%m-%dT%H:%M:%SZ")  # GNU date / BSD date fallback

METRICS_STATUS=$(curl -sf \
    -o /tmp/aer_metrics_response.json \
    -w "%{http_code}" \
    -H "X-API-Key: ${BFF_API_KEY}" \
    "${BFF_URL}/metrics?startDate=${FIVE_MIN_AGO}&endDate=${NOW}" 2>/dev/null) || METRICS_STATUS="000"

if [[ "$METRICS_STATUS" == "200" ]]; then
    METRIC_COUNT=$(python3 -c "import json,sys; d=json.load(open('/tmp/aer_metrics_response.json')); print(len(d) if isinstance(d,list) else 0)" 2>/dev/null || echo "0")
    if [[ "$METRIC_COUNT" -gt 0 ]]; then
        log_ok "BFF API returned $METRIC_COUNT metric data point(s) — pipeline is end-to-end healthy."
    else
        log_fail "BFF API returned HTTP 200 but 0 metric data points. Worker may not have processed the document yet."
        log_info "Raw response: $(cat /tmp/aer_metrics_response.json)"
    fi
else
    log_fail "BFF API /metrics returned HTTP $METRICS_STATUS."
    log_info "Raw response: $(cat /tmp/aer_metrics_response.json 2>/dev/null || echo '<empty>')"
fi
