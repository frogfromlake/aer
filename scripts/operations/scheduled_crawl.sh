#!/usr/bin/env bash
#
# SEC-041 — scheduled production crawl. The web-crawler is profile-gated out of
# the default `up` (it is a periodic job, not a long-running service), so without
# a scheduler a deployed box ingests NOTHING. This wrapper is what the host timer
# (infra/systemd/aer-crawl.timer, every 6h) invokes: it crawls every configured
# probe against the running stack using the ALREADY-BUILT image (no `--build`, so
# a scheduled run never rebuilds or pulls), then lets the normal
# Bronze→NATS→worker→Gold pipeline take over.
#
# Politeness + dedup are the crawler's own concern (ROBOTSTXT_OBEY, DOWNLOAD_DELAY,
# conditional GETs against crawler_state) — re-running every 6h is cheap (mostly
# 304s) and is how silent-edit revisions + intra-day publication rhythm get
# captured. Probes are discovered from the probes/ dir, so a new probe is picked
# up automatically with no change here.
#
# Usage (host timer, or manual): scripts/operations/scheduled_crawl.sh
# For a prod stack set COMPOSE_FILE=compose.yaml:compose.prod.yaml.
#
set -euo pipefail

cd "$(dirname "$0")/../.."

if [ ! -f .env ]; then
  echo "ERROR: .env not found in $(pwd) — cannot crawl." >&2
  exit 1
fi

PROBES_DIR="crawlers/web-crawler/probes"
if [ ! -d "$PROBES_DIR" ]; then
  echo "ERROR: ${PROBES_DIR} not found." >&2
  exit 1
fi

rc=0
for probe_path in "$PROBES_DIR"/*/; do
  [ -d "$probe_path" ] || continue
  probe="$(basename "$probe_path")"
  echo "==> $(date -u +%FT%TZ) crawling ${probe}"
  # No --build: the image is built at deploy. --rm cleans up the one-shot run.
  if docker compose --profile crawlers run --rm web-crawler --probe "$probe"; then
    echo "    ${probe}: ok"
  else
    echo "    ${probe}: FAILED (continuing with remaining probes)" >&2
    rc=1
  fi
done

if [ "$rc" -ne 0 ]; then
  echo "==> one or more probes failed; see above. (CrawlStale alert backstops a total stall.)" >&2
fi
exit "$rc"
