#!/usr/bin/env bash
# stage_secrets.sh — write the deploy-time secrets to the tmpfs staging dir.
#
# Phase 155 / ADR-046. Reads a NUL-delimited NAME\0VALUE\0... stream on stdin
# and writes each value to $AER_SECRETS_DIR/<NAME> (default /run/aer-secrets),
# mode 0600, dir 0700. On a systemd host /run is a tmpfs, so the secrets live in
# RAM only: they are gone on reboot and never touch the persistent disk — the
# whole point of Phase 155. The CD job (.github/workflows/release-images.yml)
# produces the stream from the GitHub Actions Environment Secrets and pipes it
# over SSH stdin, so the values never appear in argv (the process list) or in the
# CI log (GitHub masks registered secret values, and this script never echoes a
# value).
#
# Run on the box, e.g.:  cat stream | bash scripts/operations/stage_secrets.sh
# Idempotent: re-running overwrites the files in place.
set -euo pipefail

DIR="${AER_SECRETS_DIR:-/run/aer-secrets}"

umask 077
mkdir -p "$DIR"
chmod 700 "$DIR"

# shellcheck source=scripts/operations/_secret_synth.sh
. "$(cd "$(dirname "$0")" && pwd)/_secret_synth.sh"

count=0
# read NAME, then VALUE, both NUL-terminated. NUL can never occur inside a
# secret value, so this is robust to '=', '+', '/', spaces and newlines.
while IFS= read -r -d '' name && IFS= read -r -d '' value; do
  case "$name" in
    '' | *[!A-Za-z0-9_]*)
      echo "stage_secrets: refusing malformed secret name '$name'" >&2
      exit 1
      ;;
  esac
  if [ -z "$value" ]; then
    echo "stage_secrets: refusing empty value for '$name'" >&2
    exit 1
  fi
  # printf %s writes the value with no trailing newline; the loaders
  # (pkg/secretfile, secret_files.py, the shell resolvers) strip one anyway.
  rm -f "$DIR/$name" # a prior file is 0444 (read-only); remove before rewrite
  printf '%s' "$value" >"$DIR/$name"
  # 0444, not 0600: Docker Compose (non-swarm) ignores the secret mode/uid and
  # mounts the SOURCE file as-is, so the non-root `aer` user in our service
  # images must be able to read it. Host protection is the 0700 dir, not the mode.
  chmod 444 "$DIR/$name"
  count=$((count + 1))
done

if [ "$count" -eq 0 ]; then
  echo "stage_secrets: no secrets received on stdin — refusing to leave $DIR empty" >&2
  exit 1
fi

# Derive the DB_URL secret from the staged Postgres parts (POSTGRES_PASSWORD is
# the single Postgres secret; POSTGRES_USER / POSTGRES_DB are non-secret stream
# inputs). synth_db_url percent-encodes user+password so a credential with
# spaces / @ / : stays URL-safe. POSTGRES_USER / POSTGRES_DB are then dropped —
# they are not secrets and not compose secrets, only inputs for this derivation.
if [ -f "$DIR/POSTGRES_PASSWORD" ]; then
  _pg_user=$(cat "$DIR/POSTGRES_USER" 2>/dev/null || echo aer_admin)
  _pg_db=$(cat "$DIR/POSTGRES_DB" 2>/dev/null || echo aer_metadata)
  _pg_pass=$(cat "$DIR/POSTGRES_PASSWORD")
  rm -f "$DIR/DB_URL"
  synth_db_url "$_pg_user" "$_pg_pass" postgres 5432 "$_pg_db" >"$DIR/DB_URL"
  chmod 444 "$DIR/DB_URL"
  rm -f "$DIR/POSTGRES_USER" "$DIR/POSTGRES_DB"
  unset _pg_user _pg_db _pg_pass
fi

# Ensure every manifest secret has a file: compose `secrets: file:` requires the
# source file to exist, so an unconfigured optional secret (e.g. SMTP_PASSWORD)
# gets an empty file. A required secret that ends up empty is caught loudly by
# the consuming service's boot validator — never silently ignored.
MANIFEST="$(cd "$(dirname "$0")/../.." && pwd)/infra/secrets/secrets.manifest"
if [ -f "$MANIFEST" ]; then
  empty=0
  while IFS= read -r raw; do
    name=${raw%%#*}
    name=$(printf '%s' "$name" | tr -d '[:space:]')
    [ -n "$name" ] || continue
    case "$name" in *[!A-Za-z0-9_]*) continue ;; esac
    if [ ! -e "$DIR/$name" ]; then
      : >"$DIR/$name"
      chmod 444 "$DIR/$name"
      empty=$((empty + 1))
    fi
  done <"$MANIFEST"
  [ "$empty" -gt 0 ] &&
    echo "stage_secrets: created $empty empty file(s) for unconfigured/optional manifest secrets"
fi

echo "stage_secrets: staged $count secret file(s) into $DIR (tmpfs, 0444)"
