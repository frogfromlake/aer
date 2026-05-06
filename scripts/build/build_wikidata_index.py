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
import resource
import sqlite3
import sys
import tempfile
import time
import pickle
from collections.abc import Iterator
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
import shutil
import subprocess
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
PRED_P1082 = "http://www.wikidata.org/prop/direct/P1082"
PRED_LABEL = "http://www.w3.org/2000/01/rdf-schema#label"
PRED_ALTLABEL = "http://www.w3.org/2004/02/skos/core#altLabel"

# Predicates we collect during the hydration pass. Anything outside this
# set is dropped at parse time to keep memory and CPU bounded. P279 was
# removed in the Phase-118b post-mortem fix: the bucket DSL no longer
# walks transitive subclass closures (Wikidata's P279 graph is too wide
# on non-trivial subgraphs to trust). `wikibase:sitelinks` is also not
# collected here — the truthy-dump does not carry it; sitelinks come
# exclusively from the Pass C SPARQL hydration.
HYDRATION_PREDICATES = frozenset(
    {
        PRED_P31,
        PRED_P106,
        PRED_P1082,
        PRED_LABEL,
        PRED_ALTLABEL,
    }
)

# grep regex pattern for stream pre-filtering. Cuts down the number of
# triples Python's parser has to process by ~76% on Pass B. Empirically
# measured on a 200MB Wikidata sample: 32M raw triples → 7.2M after
# filtering. The filter is conservative — it lets through any line
# containing the predicate string, even if the string occurs in an
# object literal rather than the predicate position. False positives
# are dropped silently by the Python parser; no false negatives are
# possible because predicates always appear as exact URI strings.
GREP_FILTER_HYDRATION = (
    "prop/direct/(P31|P106|P1082)>|rdf-schema#label>|skos/core#altLabel>"
)

PROGRESS_INTERVAL = 5_000_000  # log every N triples processed in a pass
HEARTBEAT_INTERVAL_SECONDS = 60.0  # wall-clock cadence for Pass B heartbeat


# ---------------------------------------------------------------------------
# Bucket DSL
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class BucketRule:
    """One bucket from wikidata_type_buckets.yaml in evaluable form.

    All set-valued clauses are AND-combined; empty clauses are no-ops.
    `p31_subclass_of` was removed in the Phase-118b post-mortem fix —
    Wikidata's transitive P279 closure is too unreliable on non-trivial
    subgraphs (Q3152824 closes over Q565 Wikimedia Commons; Q515 closes
    over Q1055 Bundestag) to drive bucket scope. Replaced by curated
    `p31_any` lists and (for hand-picked institutions) `qid_any`.
    """

    name: str
    qid_any: frozenset[str] = frozenset()
    p31_any: frozenset[str] = frozenset()
    p106_any: frozenset[str] = frozenset()
    min_population: int | None = None
    min_sitelinks: int | None = None


def _load_buckets(path: Path) -> list[BucketRule]:
    with path.open() as f:
        raw = yaml.safe_load(f)["buckets"]
    rules: list[BucketRule] = []
    for entry in raw:
        match = entry.get("match") or {}
        # The legacy `p31_subclass_of` key is rejected loudly so an
        # accidentally-merged YAML revert cannot silently zero out
        # bucket coverage at build time.
        if "p31_subclass_of" in match:
            raise ValueError(
                f"bucket {entry.get('name')!r}: 'p31_subclass_of' was "
                "removed in Phase 118b. Replace with curated 'p31_any' "
                "or 'qid_any' lists."
            )
        rules.append(
            BucketRule(
                name=entry["name"],
                qid_any=frozenset(match.get("qid_any") or []),
                p31_any=frozenset(match.get("p31_any") or []),
                p106_any=frozenset(match.get("p106_any") or []),
                min_population=match.get("min_population"),
                min_sitelinks=entry.get("min_sitelinks"),
            )
        )
    return rules


def _bucket_matches(
    rule: BucketRule,
    qid: str,
    p31: set[str],
    p106: set[str],
    p1082_max: float | None,
    sitelinks: int | None,
) -> bool:
    """Return True iff the entity matches every declared constraint of `rule`.

    `sitelinks=None` skips the `min_sitelinks` check — used for the
    pre-Pass-C candidate filter, where sitelinks have not been hydrated
    yet. Pass real ints for the final post-Pass-C evaluation.
    """
    if rule.qid_any and qid not in rule.qid_any:
        return False
    if rule.p31_any and not (p31 & rule.p31_any):
        return False
    if rule.p106_any and not (p106 & rule.p106_any):
        return False
    if rule.min_population is not None:
        if p1082_max is None or p1082_max < rule.min_population:
            return False
    if rule.min_sitelinks is not None and sitelinks is not None:
        if sitelinks < rule.min_sitelinks:
            return False
    return True


# ---------------------------------------------------------------------------
# N-Triples streaming
# ---------------------------------------------------------------------------


def _open_dump(
    path: Path,
    predicate_filter: str | None = None,
) -> IO[bytes]:
    """Open a Wikidata N-Triples dump for streaming.

    For .bz2 files, prefers `lbzip2` (parallel multi-core decompression)
    when available — typically 3-5× faster than Python's bz2 module on
    modern multi-core systems. Falls back to bz2.open if lbzip2 is not
    installed (e.g. on the GitHub-Actions workflow path where lbzip2
    is not pre-installed).

    For plain N-Triples files, opens directly.

    If `predicate_filter` is given (a grep extended-regex), it inserts a
    grep stage between decompression and Python: only lines matching
    the regex reach the parser. This is an optimisation that cuts Pass A
    by ~6× and Pass B by ~2.5× on the full Wikidata dump, by moving
    the predicate filter out of pyoxigraph's per-line parsing into a
    native-code C grep. Empirically verified on a 200MB sample: filter
    correctness is identical to Python-side filtering (no missed
    triples, harmless false positives are silently dropped by the
    Python parser).

    The grep stage is only used together with lbzip2 — if we fell back
    to single-thread bz2.open, the bottleneck is decompression and the
    grep stage would not help (and would add subprocess overhead).

    Returns a binary file-like object yielding lines.
    """
    if str(path).endswith(".bz2"):
        if shutil.which("lbzip2"):
            if predicate_filter and shutil.which("grep"):
                log.info(
                    "using lbzip2 + grep pre-filter for parallel "
                    "decompression with predicate filter"
                )
                # Pipe: lbzip2 -dc <path> | grep -E <pattern>
                lbzip2_proc = subprocess.Popen(
                    ["lbzip2", "-dc", str(path)],
                    stdout=subprocess.PIPE,
                    stderr=subprocess.DEVNULL,
                    bufsize=1024 * 1024,
                )
                grep_proc = subprocess.Popen(
                    ["grep", "-E", predicate_filter],
                    stdin=lbzip2_proc.stdout,
                    stdout=subprocess.PIPE,
                    stderr=subprocess.DEVNULL,
                    bufsize=1024 * 1024,
                )
                # Allow lbzip2 to receive SIGPIPE if grep exits early
                assert lbzip2_proc.stdout is not None
                lbzip2_proc.stdout.close()
                assert grep_proc.stdout is not None
                return grep_proc.stdout

            log.info("using lbzip2 for parallel decompression")
            proc = subprocess.Popen(
                ["lbzip2", "-dc", str(path)],
                stdout=subprocess.PIPE,
                stderr=subprocess.DEVNULL,
                bufsize=1024 * 1024,
            )
            assert proc.stdout is not None
            return proc.stdout

        log.info("lbzip2 not found, falling back to single-thread bz2")
        return bz2.open(path, "rb")
    return open(path, "rb")


def _qid_from_uri(uri: str) -> str | None:
    """Return the bare QID/PID suffix of a Wikidata entity URI, else None."""
    if not uri.startswith(WD_ENTITY_PREFIX):
        return None
    return uri[len(WD_ENTITY_PREFIX) :]


def _iter_triples(
    dump_path: Path,
    predicate_filter: str | None = None,
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

    The optional `predicate_filter` is a grep extended-regex inserted
    upstream of Python — see `_open_dump` for the details. Pass A uses
    GREP_FILTER_P279, Pass B uses GREP_FILTER_HYDRATION.

    Truncated bz2 streams (range-request fixtures) raise EOFError from
    bz2.BZ2File; we catch it cleanly and end iteration.
    """
    n = 0
    skipped = 0
    skipped_samples: list[str] = []
    try:
        with _open_dump(dump_path, predicate_filter=predicate_filter) as f:
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
# Pass A+B cache (resume-after-failure support)
# ---------------------------------------------------------------------------
#
# Long-running local builds (8-12h for the full Wikidata truthy dump) are
# split into two phases by failure mode:
#   * Pass B: pure local I/O over a 40 GB compressed dump, ~6 h on a
#     12-core laptop. Never fails on network issues.
#   * Pass C: SPARQL hydration, vulnerable to network hiccups.
#
# To avoid losing the Pass B work on a Pass C failure, we checkpoint
# the candidate set to disk before starting Pass C. On re-run the
# script detects the checkpoint and skips directly to Pass C.
#
# The checkpoint path is derived from the output path:
#   <output>.passb_cache.pickle
#
# Lifecycle (Phase-118b post-mortem fix): the cache is no longer
# auto-deleted on successful build completion — code success is not
# the same as semantic correctness, and the user has to be able to
# re-run Pass C / final-eval against the same Pass B output after
# tweaking the YAML or `--languages`. Pass `--validated` once the
# output has been spot-checked to drop the cache.
#
# Format-version bump (1 → 2): cache now stores raw candidate
# accumulators (P31/P106/P1082/labels/altlabels), not pre-evaluated
# alias rows. The schema change reflects the architectural fix that
# moved bucket evaluation to *after* Pass C.

CACHE_FORMAT_VERSION = 3


def _cache_path(output_path: Path) -> Path:
    return output_path.with_suffix(output_path.suffix + ".passb_cache.pickle")


def _save_passb_cache(
    cache_path: Path,
    snapshot_date: str,
    languages: set[str],
    candidates: dict[str, _CandidateEntity],
) -> None:
    """Persist Pass B candidate output for resume-after-failure."""
    payload = {
        "format_version": CACHE_FORMAT_VERSION,
        "snapshot_date": snapshot_date,
        "languages": sorted(languages),
        "candidates": candidates,
    }
    tmp_path = cache_path.with_suffix(cache_path.suffix + ".tmp")
    with tmp_path.open("wb") as f:
        pickle.dump(payload, f, protocol=pickle.HIGHEST_PROTOCOL)
    os.replace(tmp_path, cache_path)
    size_mb = cache_path.stat().st_size / (1024 * 1024)
    log.info(
        "Pass B cache saved to %s (%.1f MB, %d candidates)",
        cache_path,
        size_mb,
        len(candidates),
    )


def _load_passb_cache(
    cache_path: Path,
    snapshot_date: str,
    languages: set[str],
) -> dict[str, _CandidateEntity] | None:
    """Load Pass B candidates if a compatible cache exists, else None.

    Cache is considered compatible only if snapshot_date and languages
    match exactly — guards against silently producing wrong output if
    the user changed parameters between runs.
    """
    if not cache_path.exists():
        return None
    try:
        with cache_path.open("rb") as f:
            payload = pickle.load(f)
    except (pickle.UnpicklingError, EOFError, OSError) as e:
        log.warning("Pass B cache exists but cannot be loaded: %s", e)
        log.warning("Ignoring cache; running full build")
        return None

    if payload.get("format_version") != CACHE_FORMAT_VERSION:
        log.warning(
            "Pass B cache has incompatible format version %s (expected %d); "
            "ignoring",
            payload.get("format_version"),
            CACHE_FORMAT_VERSION,
        )
        return None
    if payload.get("snapshot_date") != snapshot_date:
        log.warning(
            "Pass B cache snapshot_date=%s differs from current=%s; "
            "ignoring cache",
            payload.get("snapshot_date"),
            snapshot_date,
        )
        return None
    if set(payload.get("languages", [])) != languages:
        log.warning(
            "Pass B cache languages=%s differ from current=%s; "
            "ignoring cache",
            payload.get("languages"),
            sorted(languages),
        )
        return None

    candidates: dict[str, _CandidateEntity] = payload["candidates"]
    log.info(
        "Pass B cache loaded from %s — %d candidates, skipping Pass B, "
        "going directly to Pass C",
        cache_path,
        len(candidates),
    )
    return candidates


# ---------------------------------------------------------------------------
# Pass B — candidate accumulation
# ---------------------------------------------------------------------------
#
# Phase 118b post-mortem (2026-05): the original Pass B emitted alias rows
# and decided bucket membership in one shot. That was wrong on two axes:
#
#   * `min_sitelinks` was checked against `acc.sitelinks` from the dump,
#     but the truthy-dump carries no `wikibase:sitelinks` triples — the
#     value was always 0, and every bucket with a sitelinks threshold
#     zero-matched.
#   * `p31_subclass_of` walked Wikidata's transitive P279 graph, which
#     is unreliable on non-trivial subgraphs (Q3152824 closes over
#     Q565 Wikimedia Commons via "cultural institution").
#
# The fix: Pass B no longer evaluates buckets. It accumulates candidate
# entities — every subject that *could* match a rule on the basis of
# its dump-resident facts (P31 / P106 / P1082 / QID). Pass C then
# hydrates real sitelinks for those candidates only, and a final
# evaluation step (`_evaluate_candidates`) decides bucket membership
# with the complete picture and emits alias rows.
#
# The candidate filter is conservative — every rule's pre-Pass-C
# predicate is OR-combined. False positives at the candidate stage are
# dropped by the post-Pass-C evaluation; the only cost is one extra
# SPARQL roundtrip and a few hundred KB of label storage per dropped
# candidate.


@dataclass
class _CandidateEntity:
    qid: str
    p31: set[str] = field(default_factory=set)
    p106: set[str] = field(default_factory=set)
    p1082_max: float | None = None
    # label / altLabel: list of (value, lang) — emit one alias row per item
    labels: list[tuple[str, str]] = field(default_factory=list)
    altlabels: list[tuple[str, str]] = field(default_factory=list)


def _normalise_alias(text: str) -> str:
    """Lowercase + trim. Runtime extractor applies the same canonical form
    plus accent-folding / punctuation-stripping; storing the punctuated
    accented form here lets a single index serve all lookup variants."""
    return text.strip().lower()


def _candidate_passes_pre_filter(
    cand: _CandidateEntity,
    rules: list[BucketRule],
) -> bool:
    """True iff the candidate could match at least one rule once
    sitelinks are hydrated. Equivalent to `_bucket_matches(..., sitelinks=None)`
    OR-combined across rules."""
    return any(
        _bucket_matches(
            rule,
            qid=cand.qid,
            p31=cand.p31,
            p106=cand.p106,
            p1082_max=cand.p1082_max,
            sitelinks=None,
        )
        for rule in rules
    )


def _heartbeat(
    label: str,
    started_at: float,
    last_emit_at: float,
    triples_seen: int,
    candidates_kept: int,
) -> float:
    """Emit a wall-clock-cadenced heartbeat with throughput + RSS.

    Returns the (possibly updated) last-emit timestamp. Throttles to
    HEARTBEAT_INTERVAL_SECONDS so a tight inner loop does not flood
    the log. RSS is read via `resource.getrusage` (Linux: kB).
    """
    now = time.monotonic()
    if now - last_emit_at < HEARTBEAT_INTERVAL_SECONDS:
        return last_emit_at
    elapsed = now - started_at
    rate = triples_seen / elapsed if elapsed > 0 else 0.0
    rss_kb = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss
    log.info(
        "%s heartbeat elapsed=%.0fs triples=%d rate=%.0f/s candidates=%d rss=%.0fMB",
        label,
        elapsed,
        triples_seen,
        rate,
        candidates_kept,
        rss_kb / 1024.0,
    )
    return now


def _hydrate_candidates(
    dump_path: Path,
    rules: list[BucketRule],
    languages: set[str],
) -> dict[str, _CandidateEntity]:
    """Single sequential scan: group triples by subject, keep only subjects
    that pre-match at least one bucket rule (sitelinks-agnostic).

    Assumes `dump_path` is sorted by subject — Wikidata's
    `latest-truthy.nt.bz2` is. Memory is bounded by the candidate set
    (~10⁵ entities), not by the dump (~10⁹ triples).

    `languages` is the target-language allow-list. Labels in other
    languages are dropped at parse time rather than at evaluate time —
    Wikidata entities typically carry labels in 50-200 languages while
    AĒR uses only 3 (de/en/fr), so per-target-language filtering at
    parse time cuts per-candidate memory ~50× compared to keeping all
    languages until `_evaluate_candidates` does the same filter.
    """
    log.info("Pass B: streaming candidate hydration over %s", dump_path)
    candidates: dict[str, _CandidateEntity] = {}
    seen_subjects: set[str] = set()
    counters = {
        "subjects": 0,
        "candidates": 0,
        "labels_with_lang": 0,
        "triples": 0,
    }
    started = time.monotonic()
    last_heartbeat = started

    cur_acc: _CandidateEntity | None = None

    def flush() -> None:
        nonlocal cur_acc
        if cur_acc is None:
            return
        if _candidate_passes_pre_filter(cur_acc, rules):
            candidates[cur_acc.qid] = cur_acc
            counters["candidates"] += 1
        cur_acc = None

    for subj_uri, pred_uri, obj in _iter_triples(
        dump_path,
        predicate_filter=GREP_FILTER_HYDRATION,
    ):
        counters["triples"] += 1
        if counters["triples"] % PROGRESS_INTERVAL == 0:
            last_heartbeat = _heartbeat(
                "PassB",
                started,
                last_heartbeat,
                counters["triples"],
                counters["candidates"],
            )
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
            cur_acc = _CandidateEntity(qid=qid)
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
        elif pred_uri == PRED_LABEL:
            if (
                isinstance(obj, Literal)
                and obj.language
                and obj.language in languages
            ):
                cur_acc.labels.append((obj.value, obj.language))
                counters["labels_with_lang"] += 1
        elif pred_uri == PRED_ALTLABEL:
            if (
                isinstance(obj, Literal)
                and obj.language
                and obj.language in languages
            ):
                cur_acc.altlabels.append((obj.value, obj.language))
                counters["labels_with_lang"] += 1

    flush()
    elapsed = time.monotonic() - started
    log.info(
        "Pass B complete: subjects=%d candidates=%d labels_with_lang=%d elapsed=%.0fs",
        counters["subjects"],
        counters["candidates"],
        counters["labels_with_lang"],
        elapsed,
    )
    if counters["candidates"] == 0:
        log.warning(
            "No entities passed the candidate pre-filter. Either the bucket "
            "DSL is wrong, the dump file is not the truthy export, or the "
            "dump predicates differ from the expected URIs."
        )
    return candidates


# ---------------------------------------------------------------------------
# Final bucket evaluation (post-Pass-C)
# ---------------------------------------------------------------------------


def _evaluate_candidates(
    candidates: dict[str, _CandidateEntity],
    sitelinks: dict[str, int],
    rules: list[BucketRule],
    languages: set[str],
) -> tuple[
    dict[tuple[str, str, str], tuple[int, str]],
    dict[str, set[str]],
    dict[str, int],
]:
    """Run the full bucket DSL over hydrated candidates and emit alias rows.

    Called after Pass C has populated `sitelinks`. Candidates whose real
    sitelink count fails `min_sitelinks` are dropped here; this is where
    the Phase-118b architecture-fix bites (the old code applied the
    threshold against zeroes from the truthy dump).
    """
    alias_rows: dict[tuple[str, str, str], tuple[int, str]] = {}
    entity_buckets: dict[str, set[str]] = {}
    entity_sitelinks: dict[str, int] = {}
    matched = 0

    for qid, cand in candidates.items():
        sl = sitelinks.get(qid, 0)
        matched_buckets = [
            rule.name
            for rule in rules
            if _bucket_matches(
                rule,
                qid=qid,
                p31=cand.p31,
                p106=cand.p106,
                p1082_max=cand.p1082_max,
                sitelinks=sl,
            )
        ]
        if not matched_buckets:
            continue
        matched += 1
        for bucket_name in matched_buckets:
            entity_buckets.setdefault(qid, set()).add(bucket_name)
        entity_sitelinks[qid] = sl

        def _record(value: str, lang: str, source: str, _qid: str = qid, _sl: int = sl) -> None:
            if lang not in languages or not value:
                return
            norm = _normalise_alias(value)
            if not norm:
                return
            key = (norm, lang, _qid)
            existing = alias_rows.get(key)
            if existing is None:
                alias_rows[key] = (_sl, source)
            else:
                prev_sl, prev_src = existing
                new_src = "label" if "label" in (prev_src, source) else "altLabel"
                alias_rows[key] = (max(prev_sl, _sl), new_src)

        for value, lang in cand.labels:
            _record(value, lang, "label")
        for value, lang in cand.altlabels:
            _record(value, lang, "altLabel")

    log.info(
        "Final evaluation: candidates=%d matched=%d alias_rows=%d entities=%d",
        len(candidates),
        matched,
        len(alias_rows),
        len(entity_buckets),
    )
    if matched == 0:
        log.warning(
            "No candidates matched after Pass C. The min_sitelinks thresholds "
            "may be too strict, or sitelink hydration may have failed."
        )
    return alias_rows, entity_buckets, entity_sitelinks

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


# Transport-level exceptions that should also trigger retry. WLAN switches,
# transient TCP resets, DNS hiccups, and chunked-transfer truncation all
# raise these — and a 6-h Pass C run cannot afford to die on a 5-second
# network blip 80% of the way through. The previous decorator only
# matched the manually-raised TransientSparqlError, which left every
# requests.exceptions.* unretried.
_RETRYABLE_TRANSPORT_ERRORS = (
    TransientSparqlError,
    requests.exceptions.ConnectionError,
    requests.exceptions.Timeout,
    requests.exceptions.ChunkedEncodingError,
)


@retry(
    retry=retry_if_exception_type(_RETRYABLE_TRANSPORT_ERRORS),
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


PASS_C_CHECKPOINT_INTERVAL = 100  # write checkpoint every N batches
PASS_C_CACHE_FORMAT_VERSION = 1


def _passc_cache_path(output_path: Path) -> Path:
    return output_path.with_suffix(output_path.suffix + ".passc_cache.pickle")


def _passc_qids_signature(sorted_qids: list[str]) -> str:
    """Stable digest of the candidate QID set. Used to invalidate the
    Pass C cache if the candidate set has changed (Pass B cache differs,
    YAML changed, etc.) — without this, we could silently merge stale
    sitelinks into a fresh candidate set."""
    h = hashlib.sha256()
    for q in sorted_qids:
        h.update(q.encode())
        h.update(b"\n")
    return h.hexdigest()


def _save_passc_cache(
    cache_path: Path,
    qids_signature: str,
    sitelinks: dict[str, int],
    last_batch_completed: int,
) -> None:
    payload = {
        "format_version": PASS_C_CACHE_FORMAT_VERSION,
        "qids_signature": qids_signature,
        "sitelinks": sitelinks,
        "last_batch_completed": last_batch_completed,
    }
    tmp_path = cache_path.with_suffix(cache_path.suffix + ".tmp")
    with tmp_path.open("wb") as f:
        pickle.dump(payload, f, protocol=pickle.HIGHEST_PROTOCOL)
    os.replace(tmp_path, cache_path)


def _load_passc_cache(
    cache_path: Path,
    qids_signature: str,
) -> tuple[dict[str, int], int] | None:
    if not cache_path.exists():
        return None
    try:
        with cache_path.open("rb") as f:
            payload = pickle.load(f)
    except (pickle.UnpicklingError, EOFError, OSError) as e:
        log.warning("Pass C cache exists but cannot be loaded: %s", e)
        return None
    if payload.get("format_version") != PASS_C_CACHE_FORMAT_VERSION:
        log.warning(
            "Pass C cache has incompatible format version %s; ignoring",
            payload.get("format_version"),
        )
        return None
    if payload.get("qids_signature") != qids_signature:
        log.warning(
            "Pass C cache QID set differs from current candidates; ignoring"
        )
        return None
    return payload["sitelinks"], payload["last_batch_completed"]


def _hydrate_sitelinks_via_sparql(
    qids: list[str],
    cache_path: Path | None = None,
) -> dict[str, int]:
    """Pass C — hydrate sitelink counts for the given QIDs.

    The truthy-dump (`latest-truthy.nt.bz2`) does not include
    `wikibase:sitelinks` triples; the SPARQL endpoint does. We query
    the public endpoint with VALUES-clause batches, which are direct
    hash lookups on the QID index — fast and reliable (verified
    empirically: 500 QIDs ~ 1s).

    Returns a {qid: sitelink_count} mapping. Missing QIDs are absent
    from the returned dict (caller should default to 0).

    If `cache_path` is given, writes an incremental checkpoint every
    PASS_C_CHECKPOINT_INTERVAL batches and resumes from a compatible
    checkpoint on entry. This protects against the multi-hour Pass C
    work being lost to a sustained network outage that exceeds the
    tenacity retry budget — partial progress is durable on disk.
    """
    if not qids:
        return {}

    sorted_qids = sorted(qids)  # deterministic batch boundaries
    total_batches = (len(sorted_qids) + SPARQL_BATCH_SIZE - 1) // SPARQL_BATCH_SIZE

    sitelinks: dict[str, int] = {}
    start_batch = 0  # 0-indexed: skip first N batches when resuming

    if cache_path is not None:
        signature = _passc_qids_signature(sorted_qids)
        loaded = _load_passc_cache(cache_path, signature)
        if loaded is not None:
            sitelinks, start_batch = loaded
            log.info(
                "Pass C cache loaded from %s — resuming from batch %d/%d "
                "(%d sitelinks already hydrated)",
                cache_path,
                start_batch + 1,
                total_batches,
                len(sitelinks),
            )

    log.info(
        "Pass C: SPARQL sitelink hydration for %d QIDs in batches of %d "
        "(starting at batch %d/%d)",
        len(qids),
        SPARQL_BATCH_SIZE,
        start_batch + 1,
        total_batches,
    )

    for batch_num in range(start_batch, total_batches):
        i = batch_num * SPARQL_BATCH_SIZE
        batch = sorted_qids[i : i + SPARQL_BATCH_SIZE]
        if (batch_num + 1) % 10 == 0 or batch_num == start_batch or batch_num == total_batches - 1:
            log.info(
                "Pass C: batch %d/%d (%d QIDs)",
                batch_num + 1,
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

        # Checkpoint every N batches and on the last batch.
        if cache_path is not None and (
            (batch_num + 1) % PASS_C_CHECKPOINT_INTERVAL == 0
            or batch_num == total_batches - 1
        ):
            _save_passc_cache(
                cache_path=cache_path,
                qids_signature=_passc_qids_signature(sorted_qids),
                sitelinks=sitelinks,
                last_batch_completed=batch_num + 1,
            )

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

    cache_path = _cache_path(output_path)

    # Try to resume from a previous Pass B run. The cache is only used
    # if it matches the current snapshot_date and languages; otherwise
    # we re-run Pass B from scratch. See `_load_passb_cache`.
    candidates = _load_passb_cache(cache_path, snapshot_date, languages)

    if candidates is None:
        # Pass B: stream the dump, group by subject, keep candidates that
        # could plausibly match a bucket once sitelinks are hydrated.
        candidates = _hydrate_candidates(
            dump_path=dump_path,
            rules=rules,
            languages=languages,
        )

        # Checkpoint Pass B before starting Pass C. If Pass C fails
        # (network hiccup, SPARQL endpoint downtime, etc.), the user
        # can re-run the script and resume from this checkpoint instead
        # of repeating the multi-hour Pass B work.
        _save_passb_cache(
            cache_path=cache_path,
            snapshot_date=snapshot_date,
            languages=languages,
            candidates=candidates,
        )

    # Pass C: sitelinks hydration via SPARQL. The truthy-dump does not
    # include wikibase:sitelinks triples; we fetch them from the public
    # endpoint via VALUES-clause batched lookups (verified fast and
    # reliable in tests, ~1s per 500 QIDs). The Pass C cache makes
    # batches durable on disk every PASS_C_CHECKPOINT_INTERVAL — a
    # mid-Pass-C network outage that exceeds the tenacity retry budget
    # resumes from the last completed checkpoint rather than from
    # batch 0.
    passc_cache = _passc_cache_path(output_path)
    sitelinks = _hydrate_sitelinks_via_sparql(
        sorted(candidates.keys()),
        cache_path=passc_cache,
    )
    if passc_cache.exists():
        passc_cache.unlink()
        log.info("Pass C cache cleaned up (Pass C completed successfully)")

    # Final evaluation: now that sitelinks are real, decide bucket
    # membership and emit alias rows.
    alias_rows, entity_buckets, entity_sitelinks = _evaluate_candidates(
        candidates=candidates,
        sitelinks=sitelinks,
        rules=rules,
        languages=languages,
    )

    _write_sqlite(
        output_path=output_path,
        snapshot_date=snapshot_date,
        languages=languages,
        alias_rows=alias_rows,
        entity_buckets=entity_buckets,
        entity_sitelinks=entity_sitelinks,
    )

    # Cache is intentionally retained on successful completion. Code
    # success ≠ semantic correctness — the user must inspect the output
    # and explicitly drop the cache via `--validated` once satisfied.
    log.info(
        "Pass B cache retained at %s — run with --validated to drop it",
        cache_path,
    )

def _default_snapshot_from_mtime(dump_path: Path) -> str:
    ts = dump_path.stat().st_mtime
    return datetime.fromtimestamp(ts, tz=timezone.utc).strftime("%Y-%m-%d")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--dump-path",
        required=False,
        type=Path,
        help=(
            "Path to a Wikidata N-Triples dump. Accepts either a bz2-compressed "
            "file (`*.nt.bz2`, the production case — `latest-truthy.nt.bz2`) "
            "or a plain text file (`*.nt`, used for fixtures and local smoke "
            "tests). The streaming parser handles both transparently. "
            "Optional only when running with `--validated` (cache cleanup)."
        ),
    )
    parser.add_argument(
        "--validated",
        action="store_true",
        help=(
            "Drop the Pass B resume cache at `<output>.passb_cache.pickle` "
            "and exit. Use this once the previous build's output has been "
            "spot-checked. Without `--validated` the cache is retained on "
            "successful completion so the YAML / `--languages` can be "
            "re-applied to the same Pass B candidate set without paying the "
            "multi-hour streaming cost again."
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

    if args.validated:
        for label, path in [
            ("Pass B cache", _cache_path(args.output_path)),
            ("Pass C cache", _passc_cache_path(args.output_path)),
        ]:
            if path.exists():
                path.unlink()
                log.info("Removed %s at %s", label, path)
            else:
                log.info("No %s at %s — nothing to remove", label, path)
        return

    if args.dump_path is None:
        parser.error("--dump-path is required (omit only with --validated)")
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
