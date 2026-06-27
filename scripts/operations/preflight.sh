#!/usr/bin/env bash
#
# SEC-045 — production pre-flight secret & config validator.
#
# Run this BEFORE the first `docker compose -f compose.yaml -f compose.prod.yaml
# up -d` on a prod box. It refuses to pass if any production-critical secret is
# empty or still a placeholder, or if a required public-host/URL var is unset.
# The motivating failure: an empty GF_SECURITY_ADMIN_PASSWORD leaves Grafana on
# admin/admin — and although the prod overlay no longer routes Grafana publicly
# (SEC-001), a default-credential console reachable over the SSH tunnel is still
# unacceptable. The BFF/ingestion boot validators (config.go) cover their own
# secrets; this script covers the infra + cross-cutting vars no single service
# validates, and gives the operator ONE green/red gate before exposing TLS.
#
# Usage:
#   set -a; . ./.env; set +a; scripts/operations/preflight.sh
# or simply: make preflight
#
set -euo pipefail

# Load .env safely (NOT `. ./.env` — the WEB_CRAWLER_USER_AGENT parens parse-
# error; mirrors backup.sh / reset_validate.sh) so `make preflight` and a direct
# call behave identically.
load_env() {
  env_file="$1"
  [ -f "$env_file" ] || return 0
  while IFS= read -r _line || [ -n "$_line" ]; do
    case "$_line" in ''|\#*) continue ;; esac
    [ -z "${_line// }" ] && continue
    if [[ "$_line" =~ ^([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]]; then
      export "${BASH_REMATCH[1]}=${BASH_REMATCH[2]}"
    fi
  done < "$env_file"
  unset _line
}
load_env "$(cd "$(dirname "$0")/../.." && pwd)/.env"

# Phase 155 / ADR-046: in production, secrets are staged to a tmpfs dir at deploy
# time and override any .env value loaded above. No-op when the dir is absent.
# shellcheck source=scripts/operations/_secret_lib.sh
. "$(dirname "$0")/_secret_lib.sh"
load_secret_dir

RED='\033[1;31m'; GREEN='\033[1;32m'; YELLOW='\033[1;33m'; GRAY='\033[0;90m'; RESET='\033[0m'

fail=0
note() { echo -e "${GRAY}  · $1${RESET}"; }
bad()  { echo -e "${RED}  ✖ $1${RESET}"; fail=1; }
ok()   { echo -e "${GREEN}  ✔ $1${RESET}"; }

# A value is "unset-or-placeholder" if empty or contains the REPLACE-ME / example
# sentinels the repo ships. These must never reach a production boot.
is_unset() {
  local v="${1:-}"
  case "$v" in
    "" | *REPLACE-ME* | *replace-me* | *example.com* | your-email@example.com) return 0 ;;
    *) return 1 ;;
  esac
}

require() { # name
  local name="$1" val="${!1:-}"
  if is_unset "$val"; then bad "$name is empty or still a placeholder"; else ok "$name set"; fi
}

echo -e "${YELLOW}AĒR production pre-flight (SEC-045)${RESET}"

if [ "${APP_ENV:-}" != "production" ]; then
  echo -e "${YELLOW}APP_ENV is '${APP_ENV:-unset}', not 'production'.${RESET}"
  note "This validator is for the production overlay. Set APP_ENV=production in .env"
  note "before deploying. Refusing to give a false-green on a non-prod config."
  exit 1
fi

echo "Required secrets:"
# Infra credentials no service boot-validates centrally.
require MINIO_ROOT_USER
require MINIO_ROOT_PASSWORD
require GF_SECURITY_ADMIN_PASSWORD
require CLICKHOUSE_PASSWORD
require POSTGRES_PASSWORD

echo "Application secrets:"
require BFF_API_KEY
require INGESTION_API_KEY
require BFF_DB_PASSWORD
require BFF_AUTH_DB_PASSWORD

echo "Public host / URL coherence (SEC-036/039/040):"
require DASHBOARD_HOST
require BFF_PUBLIC_BASE_URL
require ACME_EMAIL
require ADMIN_BOOTSTRAP_EMAIL

# Cross-checks: the three host-bearing vars must agree, or links/cert/passkey break.
host="${DASHBOARD_HOST:-}"
if [ -n "$host" ]; then
  case "${BFF_PUBLIC_BASE_URL:-}" in
    "https://$host"|"https://$host/") ok "BFF_PUBLIC_BASE_URL matches DASHBOARD_HOST" ;;
    *) bad "BFF_PUBLIC_BASE_URL (${BFF_PUBLIC_BASE_URL:-unset}) should be https://$host" ;;
  esac
  if [ "${BFF_WEBAUTHN_RP_ID:-}" = "$host" ]; then ok "BFF_WEBAUTHN_RP_ID matches DASHBOARD_HOST"; else
    bad "BFF_WEBAUTHN_RP_ID (${BFF_WEBAUTHN_RP_ID:-unset}) should equal $host"; fi
fi

echo "Backup readiness (SEC-031):"
require RESTIC_REPOSITORY
require RESTIC_PASSWORD

# Localhost must not survive into a prod boot (mirrors config.go SEC-036/039).
case "${BFF_PUBLIC_BASE_URL:-}" in *localhost*) bad "BFF_PUBLIC_BASE_URL points at localhost";; esac
case "${BFF_WEBAUTHN_RP_ORIGINS:-}" in *localhost*) bad "BFF_WEBAUTHN_RP_ORIGINS contains localhost";; esac

echo
if [ "$fail" -ne 0 ]; then
  echo -e "${RED}✖ Pre-flight FAILED — fix the items above before deploying.${RESET}"
  exit 1
fi
echo -e "${GREEN}✔ Pre-flight passed. Safe to bring up the production overlay.${RESET}"
