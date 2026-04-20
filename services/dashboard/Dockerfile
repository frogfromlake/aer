# syntax=docker/dockerfile:1.7
#
# AĒR Dashboard — SvelteKit static bundle served by Nginx.
# Multi-stage per Phase 84 supply-chain hardening; no Node runtime in production.
#
# Image tags are SSoT-pinned in compose.yaml; digests are rotated via
# `make deps-refresh`. Never edit digests by hand during a feature change.

# ---------------------------------------------------------------
# 1. Build stage — Node 22 on Alpine 3.23, pnpm via Corepack.
# Upstream: https://hub.docker.com/_/node — node:22-alpine3.23
# ---------------------------------------------------------------
FROM node:22-alpine3.23@sha256:8ea2348b068a9544dae7317b4f3aafcdc032df1647bb7d768a05a5cad1a7683f AS builder

WORKDIR /workspace

# Enable Corepack + pin pnpm from package.json `packageManager` field.
RUN corepack enable

# Bring in the BFF OpenAPI bundle so the generated types match the exact
# spec this image was built against. The build context is the repo root
# (see compose.yaml) — paths below are relative to that root.
COPY services/bff-api/api/openapi.bundle.yaml /workspace/bff-openapi.bundle.yaml

WORKDIR /workspace/dashboard

# Dependency install layer (cached independently of source changes).
COPY services/dashboard/package.json services/dashboard/pnpm-lock.yaml services/dashboard/.npmrc ./
RUN --mount=type=cache,id=pnpm-store,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile

# Sources + build.
COPY services/dashboard/ ./
# SvelteKit's $env/dynamic/public is resolved at build time for the static
# adapter (ADR-020). Promote the two PUBLIC_* values used by
# src/hooks.client.ts → src/lib/observability/otel.ts to build args so the
# image is self-contained and does not depend on runtime env (Phase 98d).
# Empty PUBLIC_OTLP_ENDPOINT disables browser trace emission entirely.
ARG PUBLIC_OTLP_ENDPOINT=""
ARG PUBLIC_DEPLOYMENT_ENVIRONMENT="development"
ENV PUBLIC_OTLP_ENDPOINT=${PUBLIC_OTLP_ENDPOINT} \
    PUBLIC_DEPLOYMENT_ENVIRONMENT=${PUBLIC_DEPLOYMENT_ENVIRONMENT}
# Use the bundle we copied above so the image build is hermetic — not
# dependent on the in-tree bundle file existing at build time.
RUN pnpm exec openapi-typescript /workspace/bff-openapi.bundle.yaml -o src/lib/api/types.ts \
    && pnpm run build

# ---------------------------------------------------------------
# 2. Runtime stage — Nginx 1.29 on Alpine 3.23.
# Upstream: https://hub.docker.com/_/nginx — nginx:1.29-alpine3.23
# ---------------------------------------------------------------
FROM nginx:1.29-alpine3.23@sha256:5616878291a2eed594aee8db4dade5878cf7edcb475e59193904b198d9b830de

RUN apk upgrade --no-cache \
    && chown -R nginx:nginx /var/cache/nginx /usr/share/nginx/html \
    && sed -i 's,pid\s\+/\(var/\)\?run/nginx.pid;,pid /tmp/nginx.pid;,' /etc/nginx/nginx.conf \
    && touch /tmp/nginx.pid \
    && chown nginx:nginx /tmp/nginx.pid

# Non-root runtime user. Main process also runs as nginx (pid file and
# cache dirs are pre-chowned above so no root bootstrap is needed).
USER nginx

COPY --chown=nginx:nginx services/dashboard/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder --chown=nginx:nginx /workspace/dashboard/build /usr/share/nginx/html

# Listen on the unprivileged port 8080 (see nginx.conf). Traefik's
# loadbalancer.server.port label targets this.
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --spider -q http://127.0.0.1:8080/healthz || exit 1
