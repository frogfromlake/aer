from __future__ import annotations

from collections import Counter, defaultdict
from dataclasses import dataclass
from datetime import datetime
from typing import Iterable

import structlog

from internal.extractors.base import TimeWindow  # re-exported for backward compat

logger = structlog.get_logger()


@dataclass(frozen=True, slots=True)
class CoOccurrenceRow:
    """
    A single per-article entity-pair co-occurrence row.

    Maps 1:1 to aer_gold.entity_cooccurrences. Pair ordering is
    lexicographic on (entity_a_text, entity_b_text) so the unordered
    pair (A, B) has exactly one canonical representation.

    Phase 131a (BUG 1.5): ``window_start`` is the article's
    ``published_date``, not the sweep's wall-clock window. This makes the
    PK ``(window_start, source, article_id, A, B)`` stable across sweeps
    so re-runs collapse via ReplacingMergeTree(ingestion_version), and
    aligns the row TTL anchor (``window_start + 365 DAY``) with the rest
    of Gold (which anchors on ``published_date``). ``window_end`` mirrors
    ``window_start`` for one-article-one-row semantics.
    """

    window_start: datetime
    window_end: datetime
    source: str
    article_id: str
    entity_a_text: str
    entity_a_label: str
    entity_b_text: str
    entity_b_label: str
    cooccurrence_count: int


@dataclass(frozen=True, slots=True)
class EntityRecord:
    """Subset of aer_gold.entities used as input to the co-occurrence sweep.

    Phase 131a: ``timestamp`` carries the article's ``published_date`` so
    co-occurrence rows can be stamped per article rather than per sweep.
    """

    article_id: str
    entity_text: str
    entity_label: str
    timestamp: datetime | None = None


class EntityCoOccurrenceExtractor:
    """
    Corpus-level extractor (Phase 102) — first CorpusExtractor implementation
    per ADR-020 ("the one scope relaxation").

    For each article in the (window, source) batch, enumerates every
    unordered pair of distinct entity texts present in that article. The
    emitted ``cooccurrence_count`` is ``min(count_A, count_B)`` — binary
    in the common case, larger only when an entity surface form recurs
    multiple times within a single article. Pair ordering is lexicographic.

    The BFF aggregates by summing ``cooccurrence_count`` across articles
    in a query window (standard document-coupling network construction,
    Newman 2001), and returns top-N edges plus their incident nodes for
    the Network Science x force-directed-graph view mode.

    The extractor is pure: ``extract_pairs`` consumes records and returns
    rows. The caller (``main.corpus_extraction_loop``) is responsible for
    reading ``aer_gold.entities`` and bulk-inserting into
    ``aer_gold.entity_cooccurrences``. Idempotency is preserved by the
    ``ingestion_version`` set at sweep start; ReplacingMergeTree collapses
    duplicate (window_start, source, article_id, A, B) tuples on merge.
    """

    @property
    def name(self) -> str:
        return "entity_cooccurrence"

    def extract_pairs(
        self,
        records: Iterable[EntityRecord],
        window: TimeWindow,
        source: str,
    ) -> list[CoOccurrenceRow]:
        """Group records by article_id and emit one row per unordered pair.

        Phase 131a (BUG 1.5): the ``window`` argument bounds the sweep's
        scan range but is **not** stamped onto the emitted rows — each row
        carries the article's own ``timestamp`` so the PK
        ``(window_start, source, article_id, A, B)`` is stable across
        sweeps and ReplacingMergeTree collapses repeated emissions
        idempotently. ``window`` is retained for caller-side logging and
        to keep the legacy two-argument call sites valid.
        """
        per_article: dict[str, list[EntityRecord]] = defaultdict(list)
        for rec in records:
            if not rec.article_id or not rec.entity_text:
                continue
            per_article[rec.article_id].append(rec)

        rows: list[CoOccurrenceRow] = []
        for article_id, entities in per_article.items():
            # Per-article timestamp: pick the first non-null record timestamp
            # for this article (all records for one article share a
            # published_date). Fall back to the sweep window's start so a
            # legacy caller that does not populate the field still produces
            # rows — preserves the Phase-102 contract for unit tests.
            article_ts: datetime | None = next(
                (r.timestamp for r in entities if r.timestamp is not None), None
            )
            if article_ts is None:
                article_ts = window.start
            rows.extend(
                self._pairs_for_article(article_id, entities, article_ts, source)
            )
        return rows

    @staticmethod
    def _pairs_for_article(
        article_id: str,
        entities: list[EntityRecord],
        article_timestamp: datetime,
        source: str,
    ) -> list[CoOccurrenceRow]:
        # Collapse repeats by (text, label); count occurrences per surface form.
        counts: Counter[tuple[str, str]] = Counter()
        for rec in entities:
            counts[(rec.entity_text, rec.entity_label)] += 1

        unique = sorted(counts.keys())  # lexicographic on (text, label)
        rows: list[CoOccurrenceRow] = []
        for i in range(len(unique)):
            text_a, label_a = unique[i]
            count_a = counts[unique[i]]
            for j in range(i + 1, len(unique)):
                text_b, label_b = unique[j]
                if text_a == text_b:
                    # Same surface form, different label — skip self-pairing.
                    continue
                count_b = counts[unique[j]]
                rows.append(
                    CoOccurrenceRow(
                        window_start=article_timestamp,
                        window_end=article_timestamp,
                        source=source,
                        article_id=article_id,
                        entity_a_text=text_a,
                        entity_a_label=label_a,
                        entity_b_text=text_b,
                        entity_b_label=label_b,
                        cooccurrence_count=min(count_a, count_b),
                    )
                )
        return rows
