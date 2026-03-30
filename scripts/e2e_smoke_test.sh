#!/bin/bash
# End-to-End Smoke Test for the AĒR pipeline.
# Flow: docker compose up → healthz → ingest → wait → metrics → docker compose down
set -euo pipefail

# --- Colors ---
GREEN='\033[38;5;76m'
RED='\033[38;5;196m'
CYAN='\033[38;5;39m'
GOLD='\033[38;5;214m'
GRAY='\033[38;5;245m'
RESET='\033[0m'

# --- Config ---
INGESTION_URL="http://localhost:8081"
BFF_URL="http://localhost:8080/api/v1"
HEALTHZ_TIMEOUT=120   # seconds to wait for services to become healthy
PROCESSING_WAIT=15    # seconds to give NATS + worker time to process

PASS=0
FAIL=0

# --- Helpers ---
log_info()  { echo -e "${CYAN}[INFO]${RESET}  $*"; }
log_ok()    { echo -e "${GREEN}[PASS]${RESET}  $*"; PASS=$((PASS + 1)); }
log_fail()  { echo -e "${RED}[FAIL]${RESET}  $*"; FAIL=$((FAIL + 1)); }
log_step()  { echo -e "\n${GOLD}══ $* ${RESET}"; }

wait_for_health() {
    local name="$1"
    local url="$2"
    local deadline=$((SECONDS + HEALTHZ_TIMEOUT))
    log_info "Waiting for $name at $url ..."
    while [[ $SECONDS -lt $deadline ]]; do
        if curl -sf "$url" -o /dev/null 2>/dev/null; then
            log_ok "$name is healthy."
            return 0
        fi
        sleep 2
    done
    log_fail "$name did not become healthy within ${HEALTHZ_TIMEOUT}s."
    return 1
}

# --- Teardown (always runs) ---
cleanup() {
    log_step "Teardown"
    log_info "Running docker compose down -v ..."
    docker compose down -v --remove-orphans 2>/dev/null || true
    echo
    echo -e "${GOLD}══ Result ══════════════════════════════════${RESET}"
    echo -e "   ${GREEN}PASSED: $PASS${RESET}   ${RED}FAILED: $FAIL${RESET}"
    if [[ $FAIL -gt 0 ]]; then
        echo -e "   ${RED}Smoke test FAILED.${RESET}"
        exit 1
    else
        echo -e "   ${GREEN}Smoke test PASSED.${RESET}"
        exit 0
    fi
}
trap cleanup EXIT

# ── Step 1: Start full stack ──────────────────────────────────────────────────
log_step "Step 1: Starting full Docker Compose stack"
docker compose up -d
log_info "Stack started."

# ── Step 2: Wait for health endpoints ────────────────────────────────────────
log_step "Step 2: Health checks"
wait_for_health "Ingestion API (/healthz)" "${INGESTION_URL}/healthz"
wait_for_health "BFF API (/api/v1/healthz)" "${BFF_URL}/healthz"

# ── Step 3: Ingest a test document ───────────────────────────────────────────
log_step "Step 3: POST test document to Ingestion API"

PAYLOAD=$(cat <<'EOF'
{
  "source_id": 1,
  "documents": [
    {
      "key": "e2e-smoke-test-doc-001",
      "data": {
        "title": "E2E Smoke Test Article",
        "content": "This is an automated end-to-end smoke test document for the AER pipeline. It validates the full flow from ingestion through the NATS broker, the analysis worker, ClickHouse, and finally the BFF API.",
        "url": "https://example.com/smoke-test",
        "source": "e2e-smoke-test"
      }
    }
  ]
}
EOF
)

HTTP_STATUS=$(curl -sf -o /tmp/aer_ingest_response.json -w "%{http_code}" \
    -X POST "${INGESTION_URL}/api/v1/ingest" \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" 2>/dev/null) || HTTP_STATUS="000"

if [[ "$HTTP_STATUS" == "200" || "$HTTP_STATUS" == "207" ]]; then
    log_ok "Ingestion returned HTTP $HTTP_STATUS."
    log_info "Response: $(cat /tmp/aer_ingest_response.json)"
else
    log_fail "Ingestion returned unexpected HTTP $HTTP_STATUS."
    log_info "Response body: $(cat /tmp/aer_ingest_response.json 2>/dev/null || echo '<empty>')"
    # Continue — we still want to see what the BFF returns and to run teardown.
fi

# ── Step 4: Wait for pipeline processing ─────────────────────────────────────
log_step "Step 4: Waiting ${PROCESSING_WAIT}s for NATS + worker processing"
sleep "$PROCESSING_WAIT"

# ── Step 5: Query BFF metrics ─────────────────────────────────────────────────
log_step "Step 5: GET /api/v1/metrics (last 5 minutes)"

NOW=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
FIVE_MIN_AGO=$(date -u -d "5 minutes ago" +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null \
    || date -u -v-5M +"%Y-%m-%dT%H:%M:%SZ")  # GNU date / BSD date fallback

METRICS_STATUS=$(curl -sf \
    -o /tmp/aer_metrics_response.json \
    -w "%{http_code}" \
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

# cleanup() runs via EXIT trap
