#!/bin/bash
# Host-mode smoke test.
#
# Validates that the three services started via scripts/start.sh (ingestion-api,
# analysis-worker, bff-api) come up correctly when the binaries are run on the
# host — i.e. outside Docker Compose, reading `.env` directly.
#
# This complements scripts/e2e_smoke_test.sh, which only exercises the
# container path. The two paths share code but differ in how env vars reach
# the process (compose interpolation vs. viper/dotenv), and a divergence there
# was missed once (Phase 95 post-mortem: compose was rewriting
# INGESTION_MINIO_ACCESS_KEY → MINIO_ACCESS_KEY, host-mode was not).
#
# Preconditions:
#   1. `make up` has been run (infrastructure + debug ports + services).
#   2. At least ~5 seconds of startup grace period have elapsed.
#
# Exit code equals the number of failed checks.
set -euo pipefail

source "$(dirname "$0")/e2e_helpers.sh"

PASS=0
FAIL=0

check_http() {
    local name=$1 url=$2
    if curl -sf -o /dev/null --max-time 3 "$url"; then
        log_ok "$name healthy at $url"
    else
        log_fail "$name UNREACHABLE at $url"
    fi
}

check_process() {
    local name=$1 pidfile=$2 logfile=$3
    if [[ ! -f "$pidfile" ]]; then
        log_fail "$name pidfile missing: $pidfile"
        return
    fi
    local pid
    pid=$(cat "$pidfile")
    if kill -0 "$pid" 2>/dev/null; then
        log_ok "$name process alive (PID $pid)"
    else
        log_fail "$name process DEAD — last 20 log lines:"
        tail -n 20 "$logfile" 2>&1 | sed 's/^/    /'
    fi
}

log_step "Host-mode smoke test"

# Services need a moment after start.sh returns to actually bind ports.
sleep 5

check_http     "Ingestion API"   "http://localhost:8081/api/v1/healthz"
check_http     "BFF API"         "http://localhost:8080/api/v1/healthz"
check_process  "Analysis Worker" ".pids/worker.pid" ".pids/worker.log"

echo -e "${GOLD}══ Result ══════════════════════════════════${RESET}"
echo -e "   ${GREEN}PASSED: $PASS${RESET}   ${RED}FAILED: $FAIL${RESET}"

exit "$FAIL"
