# _secret_synth.sh — shared secret-derivation helpers (Phase 155 / ADR-046).
#
# Sourced by stage_secrets.sh (production) and stage_secrets_local.sh (dev) so the
# DB_URL secret is derived identically in both. DB_URL is NOT a stored credential:
# it is synthesised from POSTGRES_PASSWORD (the single Postgres secret) plus the
# non-secret user/host/port/db, mirroring arc42 §8.5.2.
#
# shellcheck shell=bash

# urlencode <string> — percent-encode everything except the RFC 3986 unreserved
# set, so a credential containing spaces / @ / : / / etc. is safe to embed in a
# URL userinfo. Byte-wise (LC_ALL=C) so any input encodes correctly.
urlencode() {
  local LC_ALL=C s=$1 i c
  for ((i = 0; i < ${#s}; i++)); do
    c=${s:i:1}
    case $c in
      [a-zA-Z0-9.~_-]) printf '%s' "$c" ;;
      *) printf '%%%02X' "'$c" ;;
    esac
  done
}

# synth_db_url <user> <password> <host> <port> <db> — build a libpq URL with the
# user and password percent-encoded (robust to any generated credential).
synth_db_url() {
  printf 'postgres://%s:%s@%s:%s/%s?sslmode=disable' \
    "$(urlencode "$1")" "$(urlencode "$2")" "$3" "$4" "$5"
}
