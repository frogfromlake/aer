# _secret_lib.sh — shared secret loading for AĒR host operations scripts.
#
# Phase 155 / ADR-046. In production, secrets are staged at deploy time into a
# tmpfs directory (default /run/aer-secrets), ONE file per variable, named by the
# variable; they never touch the persistent disk. This helper exports every such
# file as an environment variable, so the host scripts (backup, restore,
# verify_ilm, preflight, reset_validate) obtain their datastore / MinIO / restic
# credentials from RAM instead of a plaintext .env.
#
# No-op when the directory is absent (local dev, or a box not yet migrated), so
# the scripts stay backward-compatible with the existing load_env(.env) flow:
# call load_env(.env) FIRST (non-secret config + any local-dev values), then
# load_secret_dir so the tmpfs secrets win in production.
#
# shellcheck shell=sh

load_secret_dir() {
  _dir="${1:-${AER_SECRETS_DIR:-/run/aer-secrets}}"
  [ -d "$_dir" ] || return 0
  for _f in "$_dir"/*; do
    [ -f "$_f" ] || continue
    _name=$(basename "$_f")
    # Export only well-formed identifiers; value taken verbatim (trailing
    # newline stripped by command substitution, which is fine for secrets).
    case "$_name" in
      '' | *[!A-Za-z0-9_]*) continue ;;
    esac
    export "${_name}=$(cat "$_f")"
  done
  unset _dir _f _name
}
