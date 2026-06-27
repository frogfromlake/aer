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
docker compose --profile dashboard ps

echo "==> Deploy of ${TAG} complete — all services healthy."
