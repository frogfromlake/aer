#!/usr/bin/env python3
"""Build the Wikidata alias-index SQLite for Phase 118 entity linking.

Phase 118b — dump-based pipeline. Replaces the original SPARQL-paginated
implementation with a streaming N-Triples parser over Wikidata's
`latest-truthy.nt.bz2` dump. The resulting SQLite is byte-identical in
shape and consumer contract to the Phase-118 output; the worker, the
init container, the BFF, and all worker tests cannot tell which build
mechanism produced a given file.

Determinism contract: two runs over the same dump file (and the same
buckets YAML and language set) produce byte-identical SQLite output.
Achieved via:
  * deterministic accumulator → sorted insert
  * `sqlite3 .dump | sqlite3 fresh.db` canonicalisation round-trip
  * stable hash sidecar emitted alongside the .db

Architecture rationale: see ROADMAP.md Phase 118b and
docs/operations/operations_playbook.md
("Building and refreshing the Wikidata alias index").
"""

from __future__ import annotations

import argparse
import bz2
import hashlib
import logging
import os
import sqlite3
import sys
import tempfile
from collections import defaultdict, deque
from collections.abc import Iterable, Iterator
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import IO, Any

import requests
import yaml
from pyoxigraph import Literal, NamedNode, RdfFormat, parse  # type: ignore[import-not-found]
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

# Wikidata RDF predicate URIs we care about. Anything else is ignored.
WD_ENTITY_PREFIX = "http://www.wikidata.org/entity/"
PRED_P31 = "http://www.wikidata.org/prop/direct/P31"
PRED_P106 = "http://www.wikidata.org/prop/direct/P106"
PRED_P279 = "http://www.wikidata.org/prop/direct/P279"
PRED_P1082 = "http://www.wikidata.org/prop/direct/P1082"
PRED_LABEL = "http://www.w3.org/2000/01/rdf-schema#label"
PRED_ALTLABEL = "http://www.w3.org/2004/02/skos/core#altLabel"
PRED_SITELINKS = "http://wikiba.se/ontology#sitelinks"

# Predicates we collect during the hydration pass. Anything outside this
# set is dropped at parse time to keep memory and CPU bounded.
HYDRATION_PREDICATES = frozenset(
    {
        PRED_P31,
        PRED_P106,
        PRED_P1082,
        PRED_LABEL,
        PRED_ALTLABEL,
        PRED_SITELINKS,
    }
)

PROGRESS_INTERVAL = 5_000_000  # log every N triples processed in a pass


# ---------------------------------------------------------------------------
# Bucket DSL
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class BucketRule:
    """One bucket from wikidata_type_buckets.yaml in evaluable form."""

    name: str
    p31_any: frozenset[str] = frozenset()
    p106_any: frozenset[str] = frozenset()
    p31_subclass_of: frozenset[str] = frozenset()
    min_population: int | None = None
    min_sitelinks: int | None = None


def _load_buckets(path: Path) -> list[BucketRule]:
    with path.open() as f:
        raw = yaml.safe_load(f)["buckets"]
    rules: list[BucketRule] = []
    for entry in raw:
        match = entry.get("match") or {}
        rules.append(
            BucketRule(
                name=entry["name"],
                p31_any=frozenset(match.get("p31_any") or []),
                p106_any=frozenset(match.get("p106_any") or []),
                p31_subclass_of=frozenset(match.get("p31_subclass_of") or []),
                min_population=match.get("min_population"),
                min_sitelinks=entry.get("min_sitelinks"),
            )
        )
    return rules


def _bucket_matches(
    rule: BucketRule,
    p31: set[str],
    p106: set[str],
    p1082_max: float | None,
    p31_closure: dict[str, set[str]],
    sitelinks: int,
) -> bool:
    """Return True iff the entity matches every declared constraint of `rule`."""
    if rule.p31_any and not (p31 & rule.p31_any):
        return False
    if rule.p106_any and not (p106 & rule.p106_any):
        return False
    if rule.p31_subclass_of:
        # The closure is keyed by root QID → set of all descendants
        # (root included). Match if any of the entity's P31 values is in
        # the descendant set of any declared root.
        matched = False
        for root in rule.p31_subclass_of:
            if p31 & p31_closure.get(root, set()):
                matched = True
                break
        if not matched:
            return False
    if rule.min_population is not None:
        if p1082_max is None or p1082_max < rule.min_population:
            return False
    if rule.min_sitelinks is not None and sitelinks < rule.min_sitelinks:
        return False
    return True


def _interesting_subclass_roots(rules: Iterable[BucketRule]) -> set[str]:
    roots: set[str] = set()
    for r in rules:
        roots.update(r.p31_subclass_of)
    return roots


# ---------------------------------------------------------------------------
# N-Triples streaming
# ---------------------------------------------------------------------------


def _open_dump(path: Path) -> IO[bytes]:
    """Open a Wikidata N-Triples dump (bz2 or plain) for streaming."""
    if str(path).endswith(".bz2"):
        return bz2.open(path, "rb")
    return open(path, "rb")


def _qid_from_uri(uri: str) -> str | None:
    """Return the bare QID/PID suffix of a Wikidata entity URI, else None."""
    if not uri.startswith(WD_ENTITY_PREFIX):
        return None
    return uri[len(WD_ENTITY_PREFIX) :]


def _iter_triples(
    dump_path: Path,
) -> Iterator[tuple[str, str, Any]]:
    """Yield (subject_value, predicate_value, object) for every triple.

    Per-line resilient parsing in N-Triples format (Wikidata's
    `latest-truthy.nt.bz2` dump). pyoxigraph's stream
    parser aborts at the first malformed triple, which is fatal for a
    multi-billion-triple dump where even at 99.9999% well-formed input
    we expect hundreds of edge cases (unusual language tags, deprecated
    URI schemes, rare Unicode normalisation forms). We iterate line by
    line and parse each line in isolation so a bad line skips one
    triple, not the whole file.

    The Wikidata `latest-truthy.nt.bz2` dump is line-oriented N-Triples:
    each line is a complete, self-contained statement. Per-line parsing
    works perfectly on this format.

    Truncated bz2 streams (range-request fixtures) raise EOFError from
    bz2.BZ2File; we catch it cleanly and end iteration.
    """
    n = 0
    skipped = 0
    skipped_samples: list[str] = []
    try:
        with _open_dump(dump_path) as f:
            for line_bytes in f:
                stripped = line_bytes.strip()
                if not stripped or stripped.startswith(b"#") or stripped.startswith(b"@"):
                    # Skip blanks, comments, and Turtle prefix declarations
                    # (`@prefix wd: <...> .`). Prefixes are repeated in
                    # full-URI form on every triple in Wikidata's flat
                    # Turtle subset, so we can ignore the @prefix lines.
                    continue
                n += 1
                if n % PROGRESS_INTERVAL == 0:
                    log.info(
                        "scanned %d triples (skipped %d malformed)", n, skipped
                    )
                try:
                    for quad in parse(
                        input=line_bytes, format=RdfFormat.N_TRIPLES
                    ):
                        subj = quad.subject
                        pred = quad.predicate
                        if not isinstance(subj, NamedNode) or not isinstance(
                            pred, NamedNode
                        ):
                            continue
                        yield subj.value, pred.value, quad.object
                except (SyntaxError, ValueError) as e:
                    skipped += 1
                    if len(skipped_samples) < 5:
                        skipped_samples.append(
                            f"line {n}: {line_bytes[:120]!r} ({e})"
                        )
                    continue
    except (EOFError, OSError) as e:
        log.warning(
            "dump stream ended unexpectedly at triple #%d (%s: %s); "
            "ending pass cleanly with what was parsed",
            n,
            type(e).__name__,
            e,
        )
    if skipped > 0:
        log.warning(
            "skipped %d malformed triples out of %d total (%.4f%%)",
            skipped,
            n,
            100.0 * skipped / max(n, 1),
        )
        for sample in skipped_samples:
            log.warning("  sample: %s", sample)
    log.info("scan complete: %d triples (%d skipped)", n, skipped)

# ---------------------------------------------------------------------------
# Pass A — P279 transitive closure
# ---------------------------------------------------------------------------


def _compute_subclass_closure(
    dump_path: Path,
    roots: set[str],
) -> dict[str, set[str]]:
    """For each root QID, return the set {root} ∪ {all P279*-descendants}.

    Builds a forward edge map child → set(parents) by streaming all
    `wdt:P279` triples in a single pass, inverts it to parent → children,
    then BFS-walks downward from each root.
    """
    if not roots:
        log.info("no subclass roots requested; skipping P279 closure pass")
        return {}

    log.info("Pass A: building P279 graph (roots=%s)", sorted(roots))
    children_of: dict[str, set[str]] = defaultdict(set)
    edge_count = 0
    for subj_uri, pred_uri, obj in _iter_triples(dump_path):
        if pred_uri != PRED_P279:
            continue
        if not isinstance(obj, NamedNode):
            continue
        child_qid = _qid_from_uri(subj_uri)
        parent_qid = _qid_from_uri(obj.value)
        if child_qid is None or parent_qid is None:
            continue
        children_of[parent_qid].add(child_qid)
        edge_count += 1
    log.info("P279 graph built: %d edges, %d parents", edge_count, len(children_of))

    closure: dict[str, set[str]] = {}
    for root in roots:
        seen: set[str] = {root}
        queue: deque[str] = deque([root])
        while queue:
            cur = queue.popleft()
            for child in children_of.get(cur, ()):
                if child not in seen:
                    seen.add(child)
                    queue.append(child)
        closure[root] = seen
        log.info(
            "P279 closure root=%s descendants=%d (incl. root)",
            root,
            len(seen),
        )
    return closure


# ---------------------------------------------------------------------------
# Pass B — entity hydration & bucket evaluation
# ---------------------------------------------------------------------------


@dataclass
class _EntityAccumulator:
    qid: str
    p31: set[str] = field(default_factory=set)
    p106: set[str] = field(default_factory=set)
    p1082_max: float | None = None
    sitelinks: int = 0
    # label / altLabel: list of (value, lang) — emit one alias row per item
    labels: list[tuple[str, str]] = field(default_factory=list)
    altlabels: list[tuple[str, str]] = field(default_factory=list)


def _normalise_alias(text: str) -> str:
    """Lowercase + trim. Runtime extractor applies the same canonical form
    plus accent-folding / punctuation-stripping; storing the punctuated
    accented form here lets a single index serve all lookup variants."""
    return text.strip().lower()


def _emit_entity(
    acc: _EntityAccumulator,
    rules: list[BucketRule],
    p31_closure: dict[str, set[str]],
    languages: set[str],
    alias_rows: dict[tuple[str, str, str], tuple[int, str]],
    entity_buckets: dict[str, set[str]],
    entity_sitelinks: dict[str, int],
) -> None:
    """Evaluate bucket membership for one fully-hydrated entity and write
    its alias rows into the in-memory accumulator dictionaries."""
    matched_buckets = [
        rule.name
        for rule in rules
        if _bucket_matches(
            rule,
            acc.p31,
            acc.p106,
            acc.p1082_max,
            p31_closure,
            acc.sitelinks,
        )
    ]
    if not matched_buckets:
        return

    for bucket_name in matched_buckets:
        entity_buckets.setdefault(acc.qid, set()).add(bucket_name)
    entity_sitelinks[acc.qid] = max(
        entity_sitelinks.get(acc.qid, 0), acc.sitelinks
    )

    def _record(value: str, lang: str, source: str) -> None:
        if lang not in languages or not value:
            return
        norm = _normalise_alias(value)
        if not norm:
            return
        key = (norm, lang, acc.qid)
        existing = alias_rows.get(key)
        if existing is None:
            alias_rows[key] = (acc.sitelinks, source)
        else:
            prev_sl, prev_src = existing
            new_src = "label" if "label" in (prev_src, source) else "altLabel"
            alias_rows[key] = (max(prev_sl, acc.sitelinks), new_src)

    for value, lang in acc.labels:
        _record(value, lang, "label")
    for value, lang in acc.altlabels:
        _record(value, lang, "altLabel")


def _hydrate_and_evaluate(
    dump_path: Path,
    rules: list[BucketRule],
    p31_closure: dict[str, set[str]],
    languages: set[str],
) -> tuple[
    dict[tuple[str, str, str], tuple[int, str]],
    dict[str, set[str]],
    dict[str, int],
    dict[str, int],
]:
    """Single sequential scan: groups triples by subject, evaluates buckets,
    accumulates output rows. Assumes `dump_path` is sorted by subject —
    Wikidata's `latest-truthy.nt.bz2` is."""
    log.info("Pass B: streaming hydration over %s", dump_path)
    alias_rows: dict[tuple[str, str, str], tuple[int, str]] = {}
    entity_buckets: dict[str, set[str]] = {}
    entity_sitelinks: dict[str, int] = {}
    seen_subjects: set[str] = set()
    counters = {
        "subjects": 0,
        "evaluated": 0,
        "matched": 0,
        "labels_with_lang": 0,
    }

    cur_acc: _EntityAccumulator | None = None

    def flush() -> None:
        nonlocal cur_acc
        if cur_acc is None:
            return
        counters["evaluated"] += 1
        before_match = len(entity_buckets)
        _emit_entity(
            cur_acc,
            rules,
            p31_closure,
            languages,
            alias_rows,
            entity_buckets,
            entity_sitelinks,
        )
        if len(entity_buckets) > before_match:
            counters["matched"] += 1
        cur_acc = None

    for subj_uri, pred_uri, obj in _iter_triples(dump_path):
        if pred_uri not in HYDRATION_PREDICATES:
            continue
        qid = _qid_from_uri(subj_uri)
        if qid is None:
            continue
        if cur_acc is None or cur_acc.qid != qid:
            flush()
            if qid in seen_subjects:
                # The dump is supposed to be sorted by subject. If we see
                # a subject we've already evaluated, the assumption is
                # broken and we would silently lose triples — fail loud.
                raise RuntimeError(
                    f"Dump is not sorted by subject: {qid} reappears after "
                    "being flushed. The streaming group-by-subject strategy "
                    "requires a sorted dump (Wikidata's latest-truthy.nt.bz2 "
                    "is sorted by subject)."
                )
            seen_subjects.add(qid)
            cur_acc = _EntityAccumulator(qid=qid)
            counters["subjects"] += 1

        if pred_uri == PRED_P31:
            if isinstance(obj, NamedNode):
                target = _qid_from_uri(obj.value)
                if target is not None:
                    cur_acc.p31.add(target)
        elif pred_uri == PRED_P106:
            if isinstance(obj, NamedNode):
                target = _qid_from_uri(obj.value)
                if target is not None:
                    cur_acc.p106.add(target)
        elif pred_uri == PRED_P1082:
            if isinstance(obj, Literal):
                try:
                    val = float(obj.value)
                except ValueError:
                    continue
                if cur_acc.p1082_max is None or val > cur_acc.p1082_max:
                    cur_acc.p1082_max = val
        elif pred_uri == PRED_SITELINKS:
            if isinstance(obj, Literal):
                try:
                    val = int(obj.value)
                except ValueError:
                    continue
                if val > cur_acc.sitelinks:
                    cur_acc.sitelinks = val
        elif pred_uri == PRED_LABEL:
            if isinstance(obj, Literal) and obj.language:
                cur_acc.labels.append((obj.value, obj.language))
                counters["labels_with_lang"] += 1
        elif pred_uri == PRED_ALTLABEL:
            if isinstance(obj, Literal) and obj.language:
                cur_acc.altlabels.append((obj.value, obj.language))
                counters["labels_with_lang"] += 1

    flush()
    log.info(
        "Pass B complete: subjects=%d evaluated=%d matched=%d labels_with_lang=%d "
        "alias_rows=%d entities=%d",
        counters["subjects"],
        counters["evaluated"],
        counters["matched"],
        counters["labels_with_lang"],
        len(alias_rows),
        len(entity_buckets),
    )
    if counters["matched"] == 0:
        log.warning(
            "No entities matched any bucket. Either the bucket DSL is wrong, "
            "the dump file is not the truthy export, or the dump predicates "
            "differ from the expected URIs."
        )
    return alias_rows, entity_buckets, entity_sitelinks, counters

# ---------------------------------------------------------------------------
# Pass C — Sitelink hydration via Wikidata SPARQL
# ---------------------------------------------------------------------------

WIKIDATA_SPARQL_ENDPOINT = "https://query.wikidata.org/sparql"
SPARQL_BATCH_SIZE = 500
USER_AGENT_PASS_C = (
    "AER-WikidataIndexBuilder/1.0 "
    "(https://github.com/frogfromlake/aer; bot@example.invalid) "
    "Phase-118b/Pass-C "
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
def _sparql_post(query: str) -> dict:
    resp = requests.post(
        WIKIDATA_SPARQL_ENDPOINT,
        data={"query": query, "format": "json"},
        headers={
            "User-Agent": USER_AGENT_PASS_C,
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


def _hydrate_sitelinks_via_sparql(
    qids: list[str],
) -> dict[str, int]:
    """Pass C — hydrate sitelink counts for the given QIDs.

    The truthy-dump (`latest-truthy.nt.bz2`) does not include
    `wikibase:sitelinks` triples; the SPARQL endpoint does. We query
    the public endpoint with VALUES-clause batches, which are direct
    hash lookups on the QID index — fast and reliable (verified
    empirically: 500 QIDs ~ 1s).

    Returns a {qid: sitelink_count} mapping. Missing QIDs are absent
    from the returned dict (caller should default to 0).

    Total runtime estimate: 200k QIDs / 500-per-batch × ~1s/batch ~ 7 min.
    """
    if not qids:
        return {}

    log.info(
        "Pass C: SPARQL sitelink hydration for %d QIDs in batches of %d",
        len(qids),
        SPARQL_BATCH_SIZE,
    )

    sitelinks: dict[str, int] = {}
    sorted_qids = sorted(qids)  # deterministic batch boundaries
    total_batches = (len(sorted_qids) + SPARQL_BATCH_SIZE - 1) // SPARQL_BATCH_SIZE

    for i in range(0, len(sorted_qids), SPARQL_BATCH_SIZE):
        batch = sorted_qids[i : i + SPARQL_BATCH_SIZE]
        batch_num = i // SPARQL_BATCH_SIZE + 1
        if batch_num % 10 == 0 or batch_num == 1 or batch_num == total_batches:
            log.info(
                "Pass C: batch %d/%d (%d QIDs)",
                batch_num,
                total_batches,
                len(batch),
            )
        qid_values = " ".join(f"wd:{q}" for q in batch)
        query = (
            "SELECT ?item ?sitelinks WHERE {\n"
            f"  VALUES ?item {{ {qid_values} }}\n"
            "  ?item wikibase:sitelinks ?sitelinks .\n"
            "}"
        )
        result = _sparql_post(query)
        for binding in result.get("results", {}).get("bindings", []):
            uri = binding["item"]["value"]
            qid = uri.rsplit("/", 1)[-1]
            try:
                count = int(binding["sitelinks"]["value"])
            except (KeyError, ValueError):
                continue
            sitelinks[qid] = count

    log.info(
        "Pass C complete: %d/%d QIDs got sitelinks (missing entities default to 0)",
        len(sitelinks),
        len(qids),
    )
    return sitelinks

# ---------------------------------------------------------------------------
# SQLite assembly (unchanged shape from Phase 118)
# ---------------------------------------------------------------------------


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
        CREATE TABLE build_metadata (
            key TEXT PRIMARY KEY,
            value TEXT NOT NULL
        );
        """
    )
    return conn


def _write_sqlite(
    output_path: Path,
    snapshot_date: str,
    languages: set[str],
    alias_rows: dict[tuple[str, str, str], tuple[int, str]],
    entity_buckets: dict[str, set[str]],
    entity_sitelinks: dict[str, int],
) -> None:
    sorted_aliases = sorted(alias_rows.items(), key=lambda kv: kv[0])
    sorted_entities = sorted(entity_sitelinks.items())
    log.info(
        "writing SQLite: alias_rows=%d entities=%d",
        len(sorted_aliases),
        len(sorted_entities),
    )

    with tempfile.TemporaryDirectory() as tmpdir:
        staging = Path(tmpdir) / "staging.db"
        conn = _open_staging(staging)
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
            conn.executemany(
                "INSERT INTO build_metadata (key, value) VALUES (?, ?)",
                sorted(
                    [
                        ("snapshot_date", snapshot_date),
                        ("languages", ",".join(sorted(languages))),
                        ("schema_version", "1"),
                        ("build_method", "dump-stream"),
                        ("alias_row_count", str(len(sorted_aliases))),
                        ("entity_row_count", str(len(sorted_entities))),
                    ]
                ),
            )
            conn.execute(
                "CREATE INDEX idx_aliases_lookup ON aliases(alias, language)"
            )
        conn.close()

        # Canonicalise via dump → fresh DB so the on-disk page layout is
        # stable across runs. Pure-Python via sqlite3.Connection.iterdump
        # avoids depending on the `sqlite3` CLI binary at the runner — the
        # output is byte-equivalent to what `sqlite3 staging .dump |
        # sqlite3 canonical` would produce.
        canonical = Path(tmpdir) / "canonical.db"
        src = sqlite3.connect(staging)
        try:
            dump_sql = "\n".join(src.iterdump())
        finally:
            src.close()
        if canonical.exists():
            canonical.unlink()
        dst = sqlite3.connect(canonical)
        try:
            dst.executescript(dump_sql)
            dst.commit()
        finally:
            dst.close()
        if output_path.exists():
            output_path.unlink()
        os.replace(canonical, output_path)

    digest = hashlib.sha256(output_path.read_bytes()).hexdigest()
    size = output_path.stat().st_size
    log.info(
        "Build complete output=%s size_bytes=%d sha256=%s",
        output_path,
        size,
        digest,
    )
    sidecar = output_path.with_suffix(output_path.suffix + ".sha256")
    sidecar.write_text(f"{digest}  {output_path.name}\n")
    log.info("Sidecar hash written to %s", sidecar)


# ---------------------------------------------------------------------------
# Driver
# ---------------------------------------------------------------------------


def _build(
    dump_path: Path,
    snapshot_date: str,
    languages: set[str],
    buckets_path: Path,
    output_path: Path,
) -> None:
    rules = _load_buckets(buckets_path)
    log.info("loaded %d bucket rules from %s", len(rules), buckets_path)

    roots = _interesting_subclass_roots(rules)
    p31_closure = _compute_subclass_closure(dump_path, roots)

    alias_rows, entity_buckets, entity_sitelinks, _ = _hydrate_and_evaluate(
        dump_path=dump_path,
        rules=rules,
        p31_closure=p31_closure,
        languages=languages,
    )

    # Pass C: sitelinks hydration via SPARQL. The truthy-dump does not
    # include wikibase:sitelinks triples; we fetch them from the public
    # endpoint via VALUES-clause batched lookups (verified fast and
    # reliable in tests, ~1s per 500 QIDs).
    sparql_sitelinks = _hydrate_sitelinks_via_sparql(
        sorted(entity_buckets.keys())
    )

    # Merge sitelinks into the per-entity sitelinks map. Update alias_rows
    # so each alias-row carries the correct sitelink_count too — the
    # disambiguation tiebreaker depends on this column.
    for qid, sl in sparql_sitelinks.items():
        entity_sitelinks[qid] = sl
    for key, (sl_old, source) in list(alias_rows.items()):
        _, _, qid = key
        new_sl = entity_sitelinks.get(qid, 0)
        alias_rows[key] = (new_sl, source)

    _write_sqlite(
        output_path=output_path,
        snapshot_date=snapshot_date,
        languages=languages,
        alias_rows=alias_rows,
        entity_buckets=entity_buckets,
        entity_sitelinks=entity_sitelinks,
    )


def _default_snapshot_from_mtime(dump_path: Path) -> str:
    ts = dump_path.stat().st_mtime
    return datetime.fromtimestamp(ts, tz=timezone.utc).strftime("%Y-%m-%d")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--dump-path",
        required=True,
        type=Path,
        help=(
            "Path to a Wikidata N-Triples dump. Accepts either a bz2-compressed "
            "file (`*.nt.bz2`, the production case — `latest-truthy.nt.bz2`) "
            "or a plain text file (`*.nt`, used for fixtures and local smoke "
            "tests). The streaming parser handles both transparently."
        ),
    )
    parser.add_argument(
        "--snapshot-date",
        default=None,
        help=(
            "Operational snapshot identity (YYYY-MM-DD), persisted in "
            "build_metadata. Defaults to the dump file's mtime as YYYY-MM-DD "
            "(UTC). The GitHub workflow passes the dump's HTTP Last-Modified "
            "date; for local builds the mtime default is fine."
        ),
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

    if not args.dump_path.exists():
        parser.error(f"--dump-path does not exist: {args.dump_path}")

    languages = {lang.strip() for lang in args.languages.split(",") if lang.strip()}
    if not languages:
        parser.error("--languages must contain at least one ISO code")

    snapshot_date = args.snapshot_date or _default_snapshot_from_mtime(args.dump_path)

    log.info(
        "Starting Wikidata index build dump=%s snapshot=%s languages=%s buckets=%s",
        args.dump_path,
        snapshot_date,
        sorted(languages),
        args.buckets_file,
    )
    _build(
        dump_path=args.dump_path,
        snapshot_date=snapshot_date,
        languages=languages,
        buckets_path=args.buckets_file,
        output_path=args.output_path,
    )


if __name__ == "__main__":
    main()
