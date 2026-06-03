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
# The QID→display-label TSV rides the same image. Index images
# built before Phase 123b do not carry it at all, and a post-123b image may
# ship an empty placeholder until the next rebuild populates display labels.
# Both cases collapse to "write an empty TSV into the volume" — the
# wikidata-labels-load init treats an empty/absent file as "nothing to load
# yet", so a pre-123b pinned image must not abort the whole boot here.
if [ -f /index/wikidata_labels.tsv ]; then
    cp /index/wikidata_labels.tsv /data/wikidata/
else
    echo "wikidata-index-init: /index/wikidata_labels.tsv absent in image" \
         "— writing empty placeholder (awaiting next index rebuild)"
    : > /data/wikidata/wikidata_labels.tsv
fi
chmod 0644 \
    /data/wikidata/wikidata_aliases.db \
    /data/wikidata/wikidata_aliases.db.sha256 \
    /data/wikidata/wikidata_labels.tsv
cd /data/wikidata
sha256sum -c wikidata_aliases.db.sha256
echo "wikidata-index-init: copy + checksum verified (labels TSV: $(wc -l < wikidata_labels.tsv) rows)"
cat wikidata_aliases.db.sha256
