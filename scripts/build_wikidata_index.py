#!/usr/bin/env python3
"""Build the Wikidata alias-index SQLite for Phase 118 entity linking.

Produces a deterministic SQLite database mapping (alias, language) → QID
plus per-entity sitelink counts and bucket metadata. The output is mounted
read-only by the analysis-worker (via the wikidata-index-init compose
service) and consumed by NamedEntityExtractor for QID disambiguation.

Determinism contract: two runs with identical (snapshot_date, languages,
buckets-yaml, script-version) produce byte-identical SQLite files. The
build is therefore reproducible and the resulting sha256 is the only
artifact identity that matters at deploy time.

Architecture rationale: see docs/operations_playbook.md
("Building and refreshing the Wikidata alias index") and
docs/operations/scheduled_work.md (Category C entry).
"""

from __future__ import annotations

import argparse
import hashlib
import logging
import os
import sqlite3
import subprocess
import sys
import tempfile
from collections.abc import Iterator
from datetime import datetime, timedelta, timezone
from pathlib import Path

import requests
import yaml
from tenacity import (
    retry,
    retry_if_exception_type,
    stop_after_attempt,
    wait_exponential,
)

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(message)s",
    stream=sys.stdout,
)
log = logging.getLogger("build_wikidata_index")
WIKIDATA_SPARQL_ENDPOINT = "https://query-scholarly.wikidata.org/sparql"
PAGE_SIZE = 2000
USER_AGENT = (
    "AER-WikidataIndexBuilder/1.0 "
    "(https://github.com/frogfromlake/aer; bot@example.invalid) "
    "Python/{py}".format(py=".".join(map(str, sys.version_info[:3])))
)


class TransientSparqlError(RuntimeError):
    """Raised on 429/5xx — retried by tenacity."""


@retry(
    retry=retry_if_exception_type(TransientSparqlError),
    wait=wait_exponential(multiplier=2, min=2, max=120),
    stop=stop_after_attempt(8),
    reraise=True,
)
def _sparql(query: str) -> dict:
    resp = requests.post(
        WIKIDATA_SPARQL_ENDPOINT,
        data={"query": query, "format": "json"},
        headers={
            "User-Agent": USER_AGENT,
            "Accept": "application/sparql-results+json",
        },
        timeout=180,
    )
    if resp.status_code == 429 or 500 <= resp.status_code < 600:
        log.warning(
            "SPARQL transient error %s; will retry", resp.status_code
        )
        raise TransientSparqlError(f"HTTP {resp.status_code}: {resp.text[:200]}")
    resp.raise_for_status()
    return resp.json()


def _build_query(
    where_clause: str,
    languages: list[str],
    snapshot_date: str,
    offset: int,
) -> str:
    lang_values = ", ".join(f'"{lang}"' for lang in languages)
    return f"""
SELECT ?item ?itemLabel ?altLabel ?sitelinks WHERE {{
  {{
    SELECT ?item ?sitelinks WHERE {{
      {where_clause}
      ?item wikibase:sitelinks ?sitelinks .
      ?item schema:dateModified ?d .
      FILTER(?d <= "{snapshot_date}T00:00:00Z"^^xsd:dateTime)
    }}
    ORDER BY ?item
    LIMIT {PAGE_SIZE} OFFSET {offset}
  }}
  ?item rdfs:label ?itemLabel .
  FILTER(LANG(?itemLabel) IN ({lang_values}))
  OPTIONAL {{
    ?item skos:altLabel ?altLabel .
    FILTER(LANG(?altLabel) IN ({lang_values}))
  }}
}}
""".strip()


def _qid_from_uri(uri: str) -> str:
    # http://www.wikidata.org/entity/Q567 → Q567
    return uri.rsplit("/", 1)[-1]


def _iter_bucket_rows(
    bucket: dict,
    languages: list[str],
    snapshot_date: str,
) -> Iterator[tuple[str, int, str, str, str]]:
    """Yield (qid, sitelinks, label, lang, source) tuples for one bucket.

    `source` is "label" for rdfs:label rows and "altLabel" for
    skos:altLabel rows. The same QID may appear with multiple labels and
    altLabels — the final dedup pass collapses duplicates by primary key.
    """
    where = bucket["where_clause"]
    name = bucket["name"]
    offset = 0
    seen_pages = 0
    while True:
        query = _build_query(where, languages, snapshot_date, offset)
        log.info(
            "SPARQL bucket=%s offset=%d page_size=%d", name, offset, PAGE_SIZE
        )
        result = _sparql(query)
        rows = result.get("results", {}).get("bindings", [])
        if not rows:
            break
        for row in rows:
            qid = _qid_from_uri(row["item"]["value"])
            sitelinks = int(row["sitelinks"]["value"])
            if "itemLabel" in row:
                lit = row["itemLabel"]
                yield (
                    qid,
                    sitelinks,
                    lit["value"],
                    lit.get("xml:lang", ""),
                    "label",
                )
            if "altLabel" in row:
                lit = row["altLabel"]
                yield (
                    qid,
                    sitelinks,
                    lit["value"],
                    lit.get("xml:lang", ""),
                    "altLabel",
                )
        # Progress accounting based on outer SELECT page (deduped on ?item)
        seen_pages += 1
        if len({_qid_from_uri(r["item"]["value"]) for r in rows}) < PAGE_SIZE:
            break
        offset += PAGE_SIZE


def _open_staging(path: Path) -> sqlite3.Connection:
    if path.exists():
        path.unlink()
    conn = sqlite3.connect(path)
    conn.execute("PRAGMA journal_mode = TRUNCATE")
    conn.execute("PRAGMA synchronous = NORMAL")
    conn.executescript(
        """
        CREATE TABLE aliases (
            alias TEXT NOT NULL,
            language TEXT NOT NULL,
            wikidata_qid TEXT NOT NULL,
            sitelink_count INTEGER NOT NULL,
            alias_source TEXT NOT NULL,
            PRIMARY KEY (alias, language, wikidata_qid)
        );
        CREATE TABLE entities (
            wikidata_qid TEXT PRIMARY KEY,
            sitelink_count INTEGER NOT NULL,
            type_buckets TEXT NOT NULL
        );
        """
    )
    return conn


def _normalise_alias(text: str) -> str:
    # Lowercase + strip surrounding whitespace; the runtime extractor applies
    # the same normalisation plus punctuation-stripping / accent-folding,
    # but build-time we preserve punctuation and accents so a single index
    # can serve all three lookup variants.
    return text.strip().lower()


def _build(
    snapshot_date: str,
    languages: list[str],
    buckets_path: Path,
    output_path: Path,
) -> None:
    with buckets_path.open() as f:
        buckets = yaml.safe_load(f)["buckets"]

    # Build into a temp file first, then canonicalise via dump/restore so
    # the on-disk layout is page-stable across runs.
    with tempfile.TemporaryDirectory() as tmpdir:
        staging = Path(tmpdir) / "staging.db"
        conn = _open_staging(staging)

        # Deterministic accumulation: collect all rows in memory, sort, then
        # bulk-insert. This guarantees byte-identical output even if SPARQL
        # returns rows in a different order between runs (e.g. when ties on
        # ?item ordering are broken differently).
        # key = (normalised_alias, language, qid) → (sitelinks, source)
        # The alias is stored normalised (lowercased + stripped) so the
        # runtime lookup is a single equality probe. When a collision occurs
        # on the same primary-key tuple, the maximum sitelink count wins
        # and `label` provenance preempts `altLabel`.
        alias_rows: dict[tuple[str, str, str], tuple[int, str]] = {}
        entity_buckets: dict[str, set[str]] = {}
        entity_sitelinks: dict[str, int] = {}

        for bucket in buckets:
            for qid, sitelinks, label, lang, source in _iter_bucket_rows(
                bucket, languages, snapshot_date
            ):
                if not lang:
                    continue
                norm_alias = _normalise_alias(label)
                if not norm_alias:
                    continue
                key = (norm_alias, lang, qid)
                existing = alias_rows.get(key)
                if existing is None:
                    alias_rows[key] = (sitelinks, source)
                else:
                    prev_sl, prev_src = existing
                    new_src = "label" if "label" in (prev_src, source) else "altLabel"
                    alias_rows[key] = (max(prev_sl, sitelinks), new_src)

                entity_buckets.setdefault(qid, set()).add(bucket["name"])
                entity_sitelinks[qid] = max(
                    entity_sitelinks.get(qid, 0), sitelinks
                )

        # Sort lexicographically so insertion order is deterministic.
        sorted_aliases = sorted(alias_rows.items(), key=lambda kv: kv[0])
        sorted_entities = sorted(entity_sitelinks.items())

        with conn:
            conn.executemany(
                "INSERT INTO aliases "
                "(alias, language, wikidata_qid, sitelink_count, alias_source) "
                "VALUES (?, ?, ?, ?, ?)",
                [
                    (alias, lang, qid, sl, src)
                    for (alias, lang, qid), (sl, src) in sorted_aliases
                ],
            )
            conn.executemany(
                "INSERT INTO entities "
                "(wikidata_qid, sitelink_count, type_buckets) VALUES (?, ?, ?)",
                [
                    (
                        qid,
                        sl,
                        ",".join(sorted(entity_buckets.get(qid, {"unknown"}))),
                    )
                    for qid, sl in sorted_entities
                ],
            )
            conn.execute(
                "CREATE INDEX idx_aliases_lookup ON aliases(alias, language)"
            )
        conn.close()

        # Canonicalise: dump → fresh DB, so SQLite page layout is stable.
        dump_path = Path(tmpdir) / "staging.sql"
        with dump_path.open("w") as f:
            subprocess.run(
                ["sqlite3", str(staging), ".dump"],
                stdout=f,
                check=True,
            )
        canonical = Path(tmpdir) / "canonical.db"
        subprocess.run(
            ["sqlite3", str(canonical)],
            stdin=dump_path.open("r"),
            check=True,
        )
        if output_path.exists():
            output_path.unlink()
        os.replace(canonical, output_path)

    # Hash + size logging (used for the GitHub Actions step summary).
    digest = hashlib.sha256(output_path.read_bytes()).hexdigest()
    size = output_path.stat().st_size
    log.info(
        "Build complete output=%s size_bytes=%d sha256=%s",
        output_path,
        size,
        digest,
    )
    sidecar = output_path.with_suffix(output_path.suffix + ".sha256")
    # Standard sha256sum format ("<hash>  <basename>") so `sha256sum -c` can
    # consume the sidecar directly during image build / container start.
    sidecar.write_text(f"{digest}  {output_path.name}\n")
    log.info("Sidecar hash written to %s", sidecar)


def _default_snapshot_date() -> str:
    yesterday = datetime.now(timezone.utc) - timedelta(days=1)
    return yesterday.strftime("%Y-%m-%d")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--snapshot-date",
        default=_default_snapshot_date(),
        help="Wikidata snapshot date (YYYY-MM-DD). Default: yesterday UTC.",
    )
    parser.add_argument(
        "--languages",
        default="de,en,fr",
        help="Comma-separated language codes for label coverage.",
    )
    parser.add_argument(
        "--buckets-file",
        default="services/analysis-worker/data/wikidata_type_buckets.yaml",
        type=Path,
        help="Path to the type-bucket YAML.",
    )
    parser.add_argument(
        "--output-path",
        default="/tmp/wikidata_aliases.db",
        type=Path,
        help="Output SQLite path.",
    )
    args = parser.parse_args()

    languages = [lang.strip() for lang in args.languages.split(",") if lang.strip()]
    if not languages:
        parser.error("--languages must contain at least one ISO code")

    log.info(
        "Starting Wikidata index build snapshot=%s languages=%s buckets=%s",
        args.snapshot_date,
        languages,
        args.buckets_file,
    )
    _build(
        snapshot_date=args.snapshot_date,
        languages=languages,
        buckets_path=args.buckets_file,
        output_path=args.output_path,
    )


if __name__ == "__main__":
    main()
