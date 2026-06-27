#!/bin/sh
# wikidata-labels-load (Phase 123b) — load the QID→display-label TSV into
# aer_gold.wikidata_labels.
#
# The TSV is emitted by scripts/build/build_wikidata_index.py next to the
# alias .db and distributed into the wikidata_data volume by
# wikidata-index-init. This one-shot init reads it from the volume and
# INSERTs it; the table (created by migration 000026 via clickhouse-init) is
# a ReplacingMergeTree(updated_at): the BFF reads FINAL, so a later rebuild's
# labels win and duplicate rows from a re-run collapse on merge — read
# correctness never depends on insert-time dedup (updated_at = now() makes each
# reload a distinct block, so it is FINAL, not block-dedup, that keeps reads
# clean; do not drop FINAL in the BFF).
#
# An empty/absent TSV (the placeholder that ships until the first index
# rebuild populates display labels) is a no-op — the relabel toggle then
# simply finds no viewer-language label and every node keeps its source form.

set -e

# Phase 155 / ADR-046: resolve the <VAR>_FILE convention (Docker secrets on
# tmpfs). If CLICKHOUSE_PASSWORD_FILE points at a readable file, export the
# password from it. No-op when unset (backward-compatible with the env var).
resolve_file_secrets() {
  for _v in "$@"; do
    eval "_f=\${${_v}_FILE:-}"
    [ -n "$_f" ] || continue
    if [ ! -r "$_f" ]; then
      echo "Fatal: ${_v}_FILE is set ($_f) but unreadable" >&2
      exit 1
    fi
    IFS= read -r _val < "$_f" || true
    export "${_v}=${_val}"
  done
  unset _v _f _val
}
resolve_file_secrets CLICKHOUSE_PASSWORD

CLICKHOUSE_HOST="${CLICKHOUSE_HOST:-clickhouse}"
CLICKHOUSE_PORT="${CLICKHOUSE_PORT:-9000}"
CLICKHOUSE_USER="${CLICKHOUSE_USER:-default}"
CLICKHOUSE_PASSWORD="${CLICKHOUSE_PASSWORD:-}"
TSV="${WIKIDATA_LABELS_TSV:-/data/wikidata/wikidata_labels.tsv}"

if [ ! -s "$TSV" ]; then
    echo "wikidata-labels-load: $TSV absent or empty — nothing to load" \
         "(awaiting next index rebuild)"
    exit 0
fi

ROWS=$(wc -l < "$TSV")
echo "wikidata-labels-load: loading $ROWS label rows from $TSV"

clickhouse-client \
    --host "$CLICKHOUSE_HOST" \
    --port "$CLICKHOUSE_PORT" \
    --user "$CLICKHOUSE_USER" \
    --password "$CLICKHOUSE_PASSWORD" \
    --query "INSERT INTO aer_gold.wikidata_labels (wikidata_qid, language, label) FORMAT TabSeparatedRaw" \
    < "$TSV"

echo "wikidata-labels-load: loaded $ROWS label rows into aer_gold.wikidata_labels"
