#!/usr/bin/env bash
# Phase 118b post-mortem validation pipeline. Three stages, each
# explicitly invoked — none auto-cascades. Pass C SPARQL is real at
# every stage, so each invocation needs network access.
#
# Stages
#   sample-layer1       Extract canonical-seven + negative-control test
#                       triples from the production dump (~seconds, ~5 MB).
#   build-layer1        Build the index from the layer-1 sample and run
#                       the embedded Python validator (~5 min, one Pass C
#                       SPARQL batch).
#   determinism-layer1  Rebuild + compare hash. Confirms the determinism
#                       contract still holds with the new pipeline.
#   layer1              Convenience: sample-layer1 → build-layer1 →
#                       determinism-layer1.
#   sample-layer2       Extract a 2 GB subject-sorted prefix of the
#                       production dump (~5-10 min, catches scaling
#                       pathologies).
#   build-layer2        Build the index from the layer-2 sample
#                       (~15-20 min). Reports per-bucket counts and
#                       enforces sanity ceilings.
#   layer2              Convenience: sample-layer2 → build-layer2.
#   prod                Full production build over the entire dump
#                       (~6-7 h on a 12-core laptop). Output:
#                       $WIKIDATA_PROD_OUT (default
#                       /home/nelix/wikidata-build/wikidata_aliases.db).
#
# Inputs (override via env)
#   WIKIDATA_DUMP       /home/nelix/wikidata-build/latest-truthy.nt.bz2
#   WIKIDATA_WORKDIR    /tmp/wikidata
#   WIKIDATA_VENV       /tmp/wd_venv
#   WIKIDATA_PROD_OUT   /home/nelix/wikidata-build/wikidata_aliases.db

set -euo pipefail

DUMP="${WIKIDATA_DUMP:-/home/nelix/wikidata-build/latest-truthy.nt.bz2}"
WORKDIR="${WIKIDATA_WORKDIR:-/tmp/wikidata}"
VENV="${WIKIDATA_VENV:-/tmp/wd_venv}"
PROD_OUT="${WIKIDATA_PROD_OUT:-/home/nelix/wikidata-build/wikidata_aliases.db}"

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUCKETS="$REPO/services/analysis-worker/data/wikidata_type_buckets.yaml"
SCRIPT="$REPO/scripts/build_wikidata_index.py"
PYTHON="$VENV/bin/python"

LAYER1_SAMPLE="$WORKDIR/test_canonical.nt.bz2"
LAYER1_DB="$WORKDIR/test_canonical.db"
LAYER2_SAMPLE="$WORKDIR/test_layer2.nt.bz2"
LAYER2_DB="$WORKDIR/test_layer2.db"

# Curated test QIDs. The original Phase-118 brief listed several QIDs
# that pointed at *different entities entirely* (Q4093 = Glasgow not
# Olaf Scholz; Q1124 = Bill Clinton not Friedrich Merz; Q325817 =
# Christian V not Tagesschau; etc.). The list below has been verified
# entity-by-entity via SPARQL (rdfs:label + wdt:P31) on 2026-05-04.
#
# The trailing `>` in the regex makes match boundaries unambiguous
# (Q1 cannot match Q142, Q5 cannot match Q515, etc.).
#
#   Canonical-seven (verified QIDs, must land in correct bucket):
#     Q64       Berlin                         cities_population_threshold
#     Q61053    Olaf Scholz                    politicians
#     Q566257   Friedrich Merz                 politicians
#     Q313827   Bundesregierung (DE)           government_agencies (P31=Q35798)
#     Q154797   German Bundestag               government_agencies (P31=Q11204)
#     Q458      European Union                 eu_institutions (qid_any)
#     Q162222   Deutsche Bundesbank            central_banks (P31=Q66344)
#
#   Bucket-coverage extensions:
#     Q183      Germany                        sovereign_states (P31=Q3624078)
#     Q142      France                         sovereign_states
#     Q567      Angela Merkel                  politicians
#     Q1726     Munich                         cities_population_threshold
#     Q8889     European Commission            eu_institutions
#     Q8896     European Parliament            eu_institutions (also P31=Q11204)
#     Q8901     European Central Bank          eu_institutions (also P31=Q66344)
#
#   Out-of-scope by design (must NOT appear — TV programs / generic
#   magazines deliberately excluded; documenting absence is the test):
#     Q703907   Tagesschau (P31=Q15416 TV program)
#     Q131478   Der Spiegel (P31=Q41298 magazine)
#
#   Negative controls (must NOT appear):
#     Q565      Wikimedia Commons (P31=Q166118 archives — Phase 118
#               broadcasters relapse trap, defended by removing Q166118)
#     Q1        Universe                       (no relevant P31)
#     Q5        Human (the class itself)       (no relevant P31)
TEST_QIDS_REGEX="(Q1|Q5|Q64|Q142|Q183|Q458|Q565|Q567|Q1726|Q8889|Q8896|Q8901|Q61053|Q131478|Q154797|Q162222|Q313827|Q566257|Q703907)"

usage() {
  awk '/^# Phase 118b/,/^$/{print substr($0,3)}' "$0"
  echo
  echo "Stages: sample-layer1 build-layer1 determinism-layer1 layer1"
  echo "        sample-layer2 build-layer2 layer2"
  echo "        prod"
}

require() {
  local what="$1" path="$2"
  if [[ ! -e "$path" ]]; then
    echo "ERROR: $what not found at $path" >&2
    exit 1
  fi
}

require_inputs() {
  require "dump"   "$DUMP"
  require "venv"   "$PYTHON"
  require "script" "$SCRIPT"
  require "yaml"   "$BUCKETS"
  mkdir -p "$WORKDIR"
}

stage_sample_layer1() {
  require_inputs
  echo "[layer1] extracting test sample → $LAYER1_SAMPLE"
  lbzip2 -dc "$DUMP" \
    | LC_ALL=C grep -E "^<http://www.wikidata.org/entity/${TEST_QIDS_REGEX}>" \
    | bzip2 -c > "$LAYER1_SAMPLE"
  echo "[layer1] sample size: $(du -h "$LAYER1_SAMPLE" | cut -f1)"
  echo "[layer1] triples extracted: $(lbzip2 -dc "$LAYER1_SAMPLE" | wc -l)"
}

stage_build_layer1() {
  require_inputs
  require "layer-1 sample" "$LAYER1_SAMPLE"
  rm -f "$LAYER1_DB" "$LAYER1_DB.passb_cache.pickle" "$LAYER1_DB.sha256"
  echo "[layer1] building index"
  "$PYTHON" "$SCRIPT" \
    --dump-path "$LAYER1_SAMPLE" \
    --buckets-file "$BUCKETS" \
    --output-path "$LAYER1_DB"
  echo "[layer1] running validator"
  "$PYTHON" - "$LAYER1_DB" <<'PYEOF'
import sqlite3, sys
db = sys.argv[1]
conn = sqlite3.connect(db)

expected = {
    # Canonical seven (verified QIDs)
    "Q64": "cities_population_threshold",
    "Q61053": "politicians",
    "Q566257": "politicians",
    "Q313827": "government_agencies",
    "Q154797": "government_agencies",
    "Q458": "eu_institutions",
    "Q162222": "central_banks",
    # Coverage extensions
    "Q183": "sovereign_states",
    "Q142": "sovereign_states",
    "Q567": "politicians",
    "Q1726": "cities_population_threshold",
    "Q8889": "eu_institutions",
    "Q8896": "eu_institutions",
    "Q8901": "eu_institutions",
}
fail = 0
for qid, want in expected.items():
    row = conn.execute(
        "SELECT type_buckets FROM entities WHERE wikidata_qid=?", (qid,)
    ).fetchone()
    if row is None:
        print(f"  FAIL {qid}: not in index (expected {want})"); fail += 1
    elif want not in row[0].split(","):
        print(f"  FAIL {qid}: buckets={row[0]} (expected {want})"); fail += 1
    else:
        print(f"  PASS {qid}: {row[0]}")

# Out-of-scope by design (TV programs and generic magazines deliberately
# excluded — documenting their absence is the regression test for the
# "magazine flood" / "TV program flood" failure modes).
out_of_scope = ["Q703907", "Q131478"]
for qid in out_of_scope:
    row = conn.execute(
        "SELECT type_buckets FROM entities WHERE wikidata_qid=?", (qid,)
    ).fetchone()
    if row is None:
        print(f"  PASS {qid}: out-of-scope (not in index, expected)")
    else:
        print(f"  FAIL {qid}: leaked into buckets={row[0]} (out-of-scope)"); fail += 1

negatives = ["Q565", "Q1", "Q5"]
for qid in negatives:
    row = conn.execute(
        "SELECT type_buckets FROM entities WHERE wikidata_qid=?", (qid,)
    ).fetchone()
    if row is None:
        print(f"  PASS {qid}: not in index")
    else:
        print(f"  FAIL {qid}: leaked into buckets={row[0]}"); fail += 1

print()
print("  bucket counts:")
for tb, n in conn.execute(
    "SELECT type_buckets, COUNT(*) FROM entities "
    "GROUP BY type_buckets ORDER BY 2 DESC"
):
    print(f"    {tb}: {n}")
print(f"  total entities: {conn.execute('SELECT COUNT(*) FROM entities').fetchone()[0]}")
print(f"  total aliases:  {conn.execute('SELECT COUNT(*) FROM aliases').fetchone()[0]}")

if fail:
    sys.exit(f"\nlayer-1 validation FAILED: {fail} assertion(s)")
print("\nlayer-1 validation PASSED")
PYEOF
}

stage_determinism_layer1() {
  require_inputs
  require "layer-1 db" "$LAYER1_DB"
  echo "[layer1] determinism: capture first hash"
  local first second
  first="$(sha256sum "$LAYER1_DB" | cut -d' ' -f1)"
  rm -f "$LAYER1_DB" "$LAYER1_DB.passb_cache.pickle" "$LAYER1_DB.sha256"
  echo "[layer1] determinism: rebuild from sample"
  "$PYTHON" "$SCRIPT" \
    --dump-path "$LAYER1_SAMPLE" \
    --buckets-file "$BUCKETS" \
    --output-path "$LAYER1_DB"
  second="$(sha256sum "$LAYER1_DB" | cut -d' ' -f1)"
  echo "[layer1] hash1=$first"
  echo "[layer1] hash2=$second"
  if [[ "$first" != "$second" ]]; then
    echo "[layer1] determinism FAILED" >&2
    exit 1
  fi
  echo "[layer1] determinism OK"
}

stage_sample_layer2() {
  require_inputs
  echo "[layer2] extracting first 2 GB of decompressed dump"
  # `head -c 2G` exits as soon as the byte limit is reached and closes its
  # stdin, which makes lbzip2 receive SIGPIPE on its next write and exit
  # 141. This is the *intended* termination — head got what we asked for
  # and the upstream is correctly torn down. With `set -o pipefail` the
  # 141 would propagate as stage failure, which is misleading. Disable
  # pipefail around this one pipeline only.
  set +o pipefail
  lbzip2 -dc "$DUMP" | head -c 2G | bzip2 -c > "$LAYER2_SAMPLE"
  set -o pipefail
  echo "[layer2] sample size: $(du -h "$LAYER2_SAMPLE" | cut -f1)"
}

stage_build_layer2() {
  require_inputs
  require "layer-2 sample" "$LAYER2_SAMPLE"
  rm -f "$LAYER2_DB" "$LAYER2_DB.passb_cache.pickle" "$LAYER2_DB.sha256"
  echo "[layer2] building index"
  "$PYTHON" "$SCRIPT" \
    --dump-path "$LAYER2_SAMPLE" \
    --buckets-file "$BUCKETS" \
    --output-path "$LAYER2_DB"
  echo "[layer2] sanity check"
  "$PYTHON" - "$LAYER2_DB" <<'PYEOF'
import sqlite3, sys
conn = sqlite3.connect(sys.argv[1])
total = conn.execute("SELECT COUNT(*) FROM entities").fetchone()[0]
print(f"  total entities: {total}")
print("  bucket counts:")
counts: dict[str, int] = {}
for tb, n in conn.execute(
    "SELECT type_buckets, COUNT(*) FROM entities "
    "GROUP BY type_buckets ORDER BY 2 DESC"
):
    print(f"    {tb}: {n}")
    for b in tb.split(","):
        counts[b] = counts.get(b, 0) + n

# Sanity ceilings — these guard against the Phase-118 pathology where
# eu_institutions ballooned to ~150k via P279 closure. The 2 GB prefix
# only contains low-numbered QIDs (sorted dump), so absolute counts
# will be a fraction of the production run, but the ceilings are still
# meaningful: any over-shoot here will be enormous in production.
ceilings = {
    "eu_institutions": 30,           # curated qid_any list (28 QIDs)
    "central_banks": 200,            # P31=Q1242737 only
    "sovereign_states": 250,         # ~200 globally
}
fail = 0
for bucket, ceiling in ceilings.items():
    n = counts.get(bucket, 0)
    if n > ceiling:
        print(f"  FAIL {bucket}: {n} > ceiling {ceiling}"); fail += 1
    else:
        print(f"  PASS {bucket}: {n} ≤ ceiling {ceiling}")

if fail:
    sys.exit(f"\nlayer-2 sanity gate FAILED: {fail} bucket(s) over ceiling")
print("\nlayer-2 sanity gate PASSED")
PYEOF
}

stage_prod() {
  require_inputs
  echo "[prod] starting full build (≈ 6-7 h on 12-core laptop)"
  echo "[prod] dump:   $DUMP"
  echo "[prod] output: $PROD_OUT"
  mkdir -p "$(dirname "$PROD_OUT")"
  "$PYTHON" "$SCRIPT" \
    --dump-path "$DUMP" \
    --buckets-file "$BUCKETS" \
    --output-path "$PROD_OUT"
  echo
  echo "[prod] complete."
  echo "[prod] cache retained at ${PROD_OUT}.passb_cache.pickle"
  echo "[prod] inspect output, then run:"
  echo "         $PYTHON $SCRIPT --validated --output-path $PROD_OUT"
  echo "[prod] to drop the cache once satisfied."
}

case "${1:-help}" in
  sample-layer1)      stage_sample_layer1 ;;
  build-layer1)       stage_build_layer1 ;;
  determinism-layer1) stage_determinism_layer1 ;;
  layer1)             stage_sample_layer1; stage_build_layer1; stage_determinism_layer1 ;;
  sample-layer2)      stage_sample_layer2 ;;
  build-layer2)       stage_build_layer2 ;;
  layer2)             stage_sample_layer2; stage_build_layer2 ;;
  prod)               stage_prod ;;
  -h|--help|help|*)   usage ;;
esac
