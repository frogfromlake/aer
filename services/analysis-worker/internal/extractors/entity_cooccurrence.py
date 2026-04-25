from __future__ import annotations

from collections import Counter, defaultdict
from dataclasses import dataclass
from datetime import datetime
from typing import Iterable

import structlog

from internal.extractors.base import TimeWindow

logger = structlog.get_logger()


@dataclass(frozen=True, slots=True)
class CoOccurrenceRow:
    """
    A single per-article entity-pair co-occurrence row.

    Maps 1:1 to aer_gold.entity_cooccurrences. Pair ordering is
    lexicographic on (entity_a_text, entity_b_text) so the unordered
    pair (A, B) has exactly one canonical representation.
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
    """Subset of aer_gold.entities used as input to the co-occurrence sweep."""

    article_id: str
    entity_text: str
    entity_label: str


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
        """Group records by article_id and emit one row per unordered pair."""
        per_article: dict[str, list[EntityRecord]] = defaultdict(list)
        for rec in records:
            if not rec.article_id or not rec.entity_text:
                continue
            per_article[rec.article_id].append(rec)

        rows: list[CoOccurrenceRow] = []
        for article_id, entities in per_article.items():
            rows.extend(self._pairs_for_article(article_id, entities, window, source))
        return rows

    @staticmethod
    def _pairs_for_article(
        article_id: str,
        entities: list[EntityRecord],
        window: TimeWindow,
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
                        window_start=window.start,
                        window_end=window.end,
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
