#!/usr/bin/env bash
# config_audit.sh — production config drift detection (Phase 155 / ADR-046).
#
# Asserts the RUNNING stack's effective per-service environment matches the
# checked-in prod manifest (infra/config/prod_config_audit.manifest), reading
# each value straight from the container via `printenv`. It is the DETECTION
# layer that would have caught the 2026-06-27 drift (REVISION_DIFF_EXTRACTION_
# ENABLED + TOPIC_EXTRACTION_ENABLED silently false on the box) before a demo.
#
# Only enforces when the stack is APP_ENV=production, so it is a safe no-op on a
# dev stack (where those flags are deliberately relaxed) or when the stack is
# down. Exits non-zero on any drift; wired into `make preflight`.
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MANIFEST="${CONFIG_AUDIT_MANIFEST:-$ROOT/infra/config/prod_config_audit.manifest}"

if [ ! -f "$MANIFEST" ]; then
  echo "config-audit: manifest not found: $MANIFEST" >&2
  exit 1
fi

# Probe a long-lived service for the stack's APP_ENV. If the stack is down or not
# production, skip (this is a prod-only drift gate, not a dev check).
appenv="$(docker compose exec -T bff-api printenv APP_ENV 2>/dev/null | tr -d '\r\n' || true)"
if [ "$appenv" != "production" ]; then
  echo "config-audit: stack APP_ENV='${appenv:-<down/unknown>}' is not 'production' — skipping prod drift checks."
  exit 0
fi

fail=0
checked=0
while read -r svc var expected _rest; do
  case "$svc" in '' | \#*) continue ;; esac
  [ -n "$var" ] && [ -n "$expected" ] || continue
  actual="$(docker compose exec -T "$svc" printenv "$var" 2>/dev/null | tr -d '\r\n')"
  checked=$((checked + 1))
  if [ "$actual" = "$expected" ]; then
    echo "  OK    $svc $var=$expected"
  else
    echo "  DRIFT $svc $var: expected '$expected', got '${actual:-<unset>}'" >&2
    fail=1
  fi
done <"$MANIFEST"

if [ "$fail" -eq 0 ]; then
  echo "config-audit: $checked checks passed — no drift."
else
  echo "config-audit: DRIFT DETECTED — the running prod stack does not match the manifest." >&2
fi
exit "$fail"
