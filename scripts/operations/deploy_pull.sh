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

# Phase 155 / D9 — regenerate the non-secret .env.runtime (mounted via env_file:)
# from the box .env, so a new non-secret config var is forwarded by construction.
# Secrets are NOT here (they are staged to tmpfs and read via _FILE).
echo "==> Generating .env.runtime (non-secret config for env_file)"
bash scripts/operations/gen_runtime_env.sh

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

# Phase 155 / ADR-046 — config drift gate. Audit the freshly-deployed stack's
# effective per-service env against the checked-in prod manifest. A non-zero exit
# (drift) fails this script → fails the deploy → surfaces immediately, instead of
# shipping a box where e.g. REVISION_DIFF_EXTRACTION_ENABLED silently reverted to
# false (the 2026-06-27 incident).
echo "==> Auditing effective config against the prod manifest (drift gate)"
bash scripts/operations/config_audit.sh

# Reclaim disk from superseded image versions. Each release leaves the previous
# tag's images behind; over many deploys they fill the box (the 2026-06-28 disk
# alert was ~30 GB of stale images + build cache). `image prune -af` removes only
# images no running container references — the just-deployed stack is untouched,
# and an older tag is re-pullable from GHCR for a rollback. NOTE: this is image
# prune ONLY — never `builder prune`/`buildx prune` (Hard Rule 7: that evicts the
# HuggingFace model cache mount). The box pulls images and never builds, so it has
# no build cache to grow anyway.
echo "==> Reclaiming disk from superseded images (image prune, not build cache)"
docker image prune -af >/dev/null 2>&1 || true

echo "==> Deploy of ${TAG} complete — all services healthy."
