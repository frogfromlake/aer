#!/usr/bin/env bash
# stage_secrets_local.sh — local-dev counterpart to stage_secrets.sh (Phase 155).
#
# Reads the project .env and writes each secret named in
# infra/secrets/secrets.manifest to AER_SECRETS_DIR (default ./.aer-secrets,
# gitignored), so a local `make up` uses the exact same Docker-secrets wiring as
# production. Production stages from GitHub Environment Secrets via the CD job
# (stage_secrets.sh); this is the local equivalent, sourced from .env. DB_URL is
# synthesised from POSTGRES_* exactly as the CD job and ingestion expect.
#
# Invoked automatically by `make up` (and friends); safe to run by hand.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
ENV_FILE="${ENV_FILE:-$ROOT/.env}"
DIR="${AER_SECRETS_DIR:-$ROOT/.aer-secrets}"
MANIFEST="$ROOT/infra/secrets/secrets.manifest"

if [ ! -f "$ENV_FILE" ]; then
  echo "stage_secrets_local: $ENV_FILE not found (copy .env.example to .env first)" >&2
  exit 1
fi

# Load .env without `source` (its values — e.g. WEB_CRAWLER_USER_AGENT — contain
# characters that would break shell parsing). Split on the first '=' only.
while IFS= read -r _line || [ -n "$_line" ]; do
  case "$_line" in '' | \#*) continue ;; esac
  case "$_line" in *=*) ;; *) continue ;; esac
  _key=${_line%%=*}
  case "$_key" in [A-Za-z_]*) export "${_key}=${_line#*=}" ;; esac
done <"$ENV_FILE"
unset _line _key

# Synthesise the container-facing DB_URL (host @postgres, NOT any host-oriented
# DB_URL the .env may carry for host-side tooling). POSTGRES_PASSWORD stays the
# single Postgres secret; user+password are percent-encoded so a credential with
# spaces / @ / : is embedded safely (shared with the CD path via _secret_synth).
# shellcheck source=scripts/operations/_secret_synth.sh
. "$(dirname "$0")/_secret_synth.sh"
DB_URL="$(synth_db_url "${POSTGRES_USER:-aer_admin}" "${POSTGRES_PASSWORD:-}" postgres 5432 "${POSTGRES_DB:-aer_metadata}")"
export DB_URL

umask 077
mkdir -p "$DIR"
chmod 700 "$DIR"

count=0
empty=0
# Invariant: every manifest secret gets a file, EMPTY when unconfigured (an
# optional secret like SMTP_PASSWORD). compose `secrets: file:` requires the
# file to exist; an empty required secret is caught by the service's boot
# validator, never silently ignored.
while IFS= read -r _raw; do
  # first whitespace-delimited token, comments stripped
  _name=${_raw%%#*}
  _name=$(printf '%s' "$_name" | tr -d '[:space:]')
  [ -n "$_name" ] || continue
  case "$_name" in *[!A-Za-z0-9_]*) continue ;; esac
  eval "_val=\${$_name-}"
  rm -f "$DIR/$_name" # the prior file is 0444 (read-only); remove before rewrite
  printf '%s' "$_val" >"$DIR/$_name"
  # 0444, not 0600: Docker Compose (non-swarm) ignores the secret mode/uid and
  # mounts the SOURCE file as-is, so a non-root container user (the `aer` uid in
  # our service images) must be able to read it. Host-side protection comes from
  # the 0700 directory (only its owner can traverse), not the file mode.
  chmod 444 "$DIR/$_name"
  count=$((count + 1))
  [ -n "$_val" ] || empty=$((empty + 1))
done <"$MANIFEST"
unset _raw _name _val

echo "stage_secrets_local: staged $count secret file(s) into $DIR (${empty} empty/optional)"
