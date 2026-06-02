#!/bin/sh
# wikidata-index-init: copy the prebuilt SQLite alias index from the image
# layer at /index/ into the named volume mounted at /data/wikidata, then
# verify the checksum so a corrupted volume aborts the boot loudly.
#
# Mirrors infra/minio/setup.sh's pattern (separate file, bind-mounted into
# the container). The earlier inline form combined a custom entrypoint
# with a YAML folded-scalar command body and produced a containerd zombie
# state on completion — `docker compose up --wait` then blocked for 10+
# minutes per reset waiting for service_completed_successfully. See
# Phase 120b roadmap entry.

set -ex

cp /index/wikidata_aliases.db /data/wikidata/
cp /index/wikidata_aliases.db.sha256 /data/wikidata/
# Phase 123b: the QID→display-label TSV rides the same image. It may be an
# empty placeholder until the next index rebuild populates it; the
# wikidata-labels-load init treats an empty file as "nothing to load yet".
cp /index/wikidata_labels.tsv /data/wikidata/
chmod 0644 \
    /data/wikidata/wikidata_aliases.db \
    /data/wikidata/wikidata_aliases.db.sha256 \
    /data/wikidata/wikidata_labels.tsv
cd /data/wikidata
sha256sum -c wikidata_aliases.db.sha256
echo "wikidata-index-init: copy + checksum verified (labels TSV: $(wc -l < wikidata_labels.tsv) rows)"
cat wikidata_aliases.db.sha256
