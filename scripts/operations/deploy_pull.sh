#!/usr/bin/env bash
#
# M4 (Phase 150) — the production deploy step that runs ON THE BOX: pin the
# release tag, pull the pre-built GHCR images, and recreate the stack, gated on
# every service's healthcheck. Invoked by .github/workflows/release-images.yml
# over SSH after a vX.Y.Z tag's images are published, and runnable by hand for a
# manual deploy or a rollback to an older tag:
#
#   bash scripts/operations/deploy_pull.sh v1.2.3      # deploy / roll back to v1.2.3
#
# Prerequisites on the box (one-time): `docker login ghcr.io` with a read-only
# read:packages token (M3a), and COMPOSE_FILE=compose.yaml:compose.prod.yaml in
# .env so the prod overlay (image-pull, !reset build) is active. The box builds
# nothing — it only pulls.
set -euo pipefail

TAG="${1:?usage: deploy_pull.sh <image-tag>   (e.g. v1.2.3 or latest)}"
cd "$(cd "$(dirname "$0")/../.." && pwd)"   # repo root (= /opt/aer on the box)

echo "==> Pinning IMAGE_TAG=${TAG} in .env (so manual compose calls match the deploy)"
if grep -q '^IMAGE_TAG=' .env; then
  sed -i "s|^IMAGE_TAG=.*|IMAGE_TAG=${TAG}|" .env
else
  echo "IMAGE_TAG=${TAG}" >> .env
fi

echo "==> Pulling pre-built images for ${TAG}"
docker compose --profile dashboard pull

echo "==> Recreating the stack (waiting for every healthcheck to pass)"
# --wait exits non-zero if any service fails its healthcheck within the timeout,
# so a broken deploy fails the SSH command → fails the GitHub Actions job →
# surfaces immediately instead of silently shipping an unhealthy stack.
docker compose --profile dashboard up -d --remove-orphans --wait --wait-timeout 300

# Observability config (Prometheus alert rules, Grafana provisioning of dashboards
# + alert rules) is BIND-MOUNTED, not baked into images — so the `up --wait` above
# does NOT recreate prometheus/grafana when only a mounted file changed, and a
# tweaked alert rule would silently never load. If this release touched
# infra/observability/, recreate just those two readers so the new config applies.
# A per-box state file records the last-deployed commit to scope this to releases
# that really changed that config (a missing/unknown previous state recreates, the
# safe default). The file is untracked + survives `git checkout --force`.
STATE_FILE=".aer-deployed-sha"
NEW_SHA="$(git rev-parse HEAD)"
PREV_SHA="$( [[ -f "$STATE_FILE" ]] && cat "$STATE_FILE" || true )"
if [[ -n "$PREV_SHA" ]] && git cat-file -e "$PREV_SHA" 2>/dev/null \
   && git diff --quiet "$PREV_SHA" "$NEW_SHA" -- infra/observability/; then
  echo "==> infra/observability/ unchanged since last deploy — no prometheus/grafana reload needed"
else
  echo "==> infra/observability/ changed (or first deploy) — reloading prometheus + grafana (bind-mounted config)"
  docker compose --profile dashboard up -d --no-deps --force-recreate prometheus grafana
fi
echo "$NEW_SHA" > "$STATE_FILE"

docker compose --profile dashboard ps

echo "==> Deploy of ${TAG} complete — all services healthy."
