"""Tests for the Phase 118 Wikidata entity-linking integration.

Covers WikidataAliasIndex (the SQLite lookup) and NamedEntityExtractor's
linked-row emission. Uses an in-memory ~50-entity SQLite fixture — never
the production index, which is a build artefact.
"""

from __future__ import annotations

import sqlite3
from datetime import datetime, timezone
from pathlib import Path
from typing import Iterable

import pytest

from internal.extractors.entity_linking import (
    CONFIDENCE_ACCENT_FOLD,
    CONFIDENCE_ALIAS,
    CONFIDENCE_EXACT,
    WikidataAliasIndex,
)


# ---------------------------------------------------------------------------
# Fixture index — ~50 known entities, hand-curated for deterministic tests
# ---------------------------------------------------------------------------

# (alias, language, qid, sitelinks, alias_source)
_FIXTURE_ROWS: tuple[tuple[str, str, str, int, str], ...] = (
    # Angela Merkel (Q567) and aliases
    ("angela merkel", "de", "Q567", 220, "label"),
    ("merkel", "de", "Q567", 220, "altLabel"),
    ("bundeskanzlerin merkel", "de", "Q567", 220, "altLabel"),
    ("angela merkel", "en", "Q567", 220, "label"),
    # Berlin the city (Q64) — clear sitelink lead
    ("berlin", "de", "Q64", 350, "label"),
    ("berlin", "en", "Q64", 350, "label"),
    ("berlin", "fr", "Q64", 350, "label"),
    # Berlin the surname (Q4878789) — collision target with low sitelink count
    # in `de`, but in `en` should still lose to the city via sitelink count.
    ("berlin", "de", "Q4878789", 5, "altLabel"),
    ("berlin", "en", "Q4878789", 5, "altLabel"),
    # Olaf Scholz (Q4093)
    ("olaf scholz", "de", "Q4093", 110, "label"),
    ("scholz", "de", "Q4093", 110, "altLabel"),
    # Friedrich Merz (Q1124)
    ("friedrich merz", "de", "Q1124", 90, "label"),
    ("merz", "de", "Q1124", 90, "altLabel"),
    # Tagesschau (Q325817)
    ("tagesschau", "de", "Q325817", 35, "label"),
    # Bundesregierung (Q3168143)
    ("bundesregierung", "de", "Q3168143", 28, "label"),
    # Bundestag (Q1055)
    ("bundestag", "de", "Q1055", 70, "label"),
    # Europäische Union (Q458) — exact match in de, accent fold target in fr
    ("europäische union", "de", "Q458", 280, "label"),
    ("union européenne", "fr", "Q458", 280, "label"),
    ("european union", "en", "Q458", 280, "label"),
    # A few decoy entities to make the table non-trivial
    ("paris", "fr", "Q90", 320, "label"),
    ("paris", "en", "Q90", 320, "label"),
    ("paris", "de", "Q90", 320, "label"),
    ("london", "en", "Q84", 340, "label"),
    ("london", "de", "Q84", 340, "label"),
    ("rom", "de", "Q220", 300, "label"),
    ("madrid", "de", "Q2807", 290, "label"),
    ("amsterdam", "de", "Q727", 270, "label"),
    ("wien", "de", "Q1741", 260, "label"),
    ("zürich", "de", "Q72", 240, "label"),
    # Accent-fold target: French city "Lyon" referenced as "lyon" without accent.
    # Add an alias variant that REQUIRES accent folding to match.
    ("lyôn", "fr", "Q456", 200, "label"),
)


def _build_fixture_index(path: Path) -> None:
    conn = sqlite3.connect(path)
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
        CREATE INDEX idx_aliases_lookup ON aliases(alias, language);
        """
    )
    conn.executemany(
        "INSERT OR IGNORE INTO aliases VALUES (?, ?, ?, ?, ?)",
        _FIXTURE_ROWS,
    )
    seen_qids: set[str] = set()
    for _alias, _lang, qid, sl, _src in _FIXTURE_ROWS:
        if qid in seen_qids:
            continue
        seen_qids.add(qid)
        conn.execute(
            "INSERT OR IGNORE INTO entities VALUES (?, ?, ?)",
            (qid, sl, "test"),
        )
    conn.commit()
    conn.close()


@pytest.fixture
def fixture_index(tmp_path: Path) -> WikidataAliasIndex:
    db_path = tmp_path / "wikidata_test.db"
    _build_fixture_index(db_path)
    return WikidataAliasIndex(db_path)


# ---------------------------------------------------------------------------
# WikidataAliasIndex — direct lookup tests
# ---------------------------------------------------------------------------


def test_exact_label_match_returns_qid(fixture_index: WikidataAliasIndex) -> None:
    candidate = fixture_index.lookup("Angela Merkel", "de")
    assert candidate is not None
    assert candidate.wikidata_qid == "Q567"
    assert candidate.method == "exact_match"
    assert candidate.confidence == CONFIDENCE_EXACT


def test_alt_label_match_returns_lower_confidence(
    fixture_index: WikidataAliasIndex,
) -> None:
    candidate = fixture_index.lookup("Merkel", "de")
    assert candidate is not None
    assert candidate.wikidata_qid == "Q567"
    assert candidate.method == "alias_lookup"
    assert candidate.confidence == CONFIDENCE_ALIAS


def test_unknown_entity_returns_none(fixture_index: WikidataAliasIndex) -> None:
    assert fixture_index.lookup("Definitely Not A Real Entity", "de") is None


def test_sitelink_tiebreak_prefers_higher_count(
    fixture_index: WikidataAliasIndex,
) -> None:
    # Berlin city (350 sitelinks, label) wins over Berlin surname
    # (5 sitelinks, altLabel). After the Tier-1.6 refinement (2026-05-04)
    # the lookup ranks across both alias sources by sitelink count, so
    # this test now guards the unified-source sitelink ordering rather
    # than the old label-preempts-altLabel tier behaviour.
    candidate = fixture_index.lookup("Berlin", "de")
    assert candidate is not None
    assert candidate.wikidata_qid == "Q64"
    assert candidate.method == "exact_match"


def test_high_sitelink_altlabel_beats_low_sitelink_label(tmp_path: Path) -> None:
    # Phase-118b post-mortem regression test for the Bundestag misrouting
    # discovered on 2026-05-04. Before the Tier-1.6 fix, the strict
    # label > altLabel tier ordering preferred Q547751 ("Federal
    # Convention 1815-1848", `Bundestag` as primary label, 13 sitelinks)
    # over Q154797 (modern German Bundestag, `Bundestag` as altLabel,
    # 90 sitelinks) — wrong for the news-domain. The unified-source
    # sitelink-tiebreaker resolves this in favour of the modern entity.
    db = tmp_path / "tier16.db"
    conn = sqlite3.connect(db)
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
        CREATE TABLE entities (wikidata_qid TEXT PRIMARY KEY, sitelink_count INTEGER NOT NULL, type_buckets TEXT NOT NULL);
        CREATE TABLE build_metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL);
        """
    )
    conn.executemany(
        "INSERT INTO aliases (alias, language, wikidata_qid, sitelink_count, alias_source) VALUES (?, ?, ?, ?, ?)",
        [
            ("bundestag", "de", "Q547751", 13, "label"),
            ("bundestag", "de", "Q154797", 90, "altLabel"),
        ],
    )
    conn.commit()
    conn.close()
    idx = WikidataAliasIndex(db)
    candidate = idx.lookup("Bundestag", "de")
    assert candidate is not None
    assert candidate.wikidata_qid == "Q154797"
    assert candidate.method == "alias_lookup"
    assert candidate.confidence == CONFIDENCE_ALIAS
    idx.close()


def test_language_scoping_isolates_collisions(
    fixture_index: WikidataAliasIndex,
) -> None:
    # Identical surface form, different language scopes both resolve via
    # label-tier match to the city — the surname row exists in `en` as
    # altLabel only, so it cannot beat the city in either language.
    de = fixture_index.lookup("Berlin", "de")
    en = fixture_index.lookup("Berlin", "en")
    assert de is not None and en is not None
    assert de.wikidata_qid == "Q64" == en.wikidata_qid


def test_punctuation_and_case_normalisation(
    fixture_index: WikidataAliasIndex,
) -> None:
    candidate = fixture_index.lookup("  ANGELA MERKEL!  ", "de")
    assert candidate is not None
    assert candidate.wikidata_qid == "Q567"
    assert candidate.method == "exact_match"


def test_accent_fold_only_for_configured_languages(
    fixture_index: WikidataAliasIndex,
) -> None:
    # `Lyon` (no accent) must accent-fold to match `Lyôn` in fr.
    candidate = fixture_index.lookup("Lyon", "fr")
    assert candidate is not None
    assert candidate.wikidata_qid == "Q456"
    assert candidate.method == "accent_fold"
    assert candidate.confidence == CONFIDENCE_ACCENT_FOLD

    # The same accent-fold attempt in `de` returns None — accent folding is
    # off for German.
    assert fixture_index.lookup("Lyon", "de") is None


def test_empty_inputs_return_none(fixture_index: WikidataAliasIndex) -> None:
    assert fixture_index.lookup("", "de") is None
    assert fixture_index.lookup("Berlin", "") is None


# ---------------------------------------------------------------------------
# NamedEntityExtractor with alias_index — pipeline integration
# ---------------------------------------------------------------------------


def _silver_core(text: str, language: str = "de"):
    from internal.models import SilverCore

    return SilverCore(
        document_id="test-doc-1",
        source="tagesschau",
        source_type="rss",
        raw_text=text,
        cleaned_text=text,
        language=language,
        timestamp=datetime(2026, 5, 1, tzinfo=timezone.utc),
        url="https://example.invalid/article",
        schema_version=1,
        word_count=len(text.split()),
    )


@pytest.fixture
def stubbed_extractor(monkeypatch, fixture_index):
    """NamedEntityExtractor with spaCy replaced by a hand-rolled stub.

    Loading the real de_core_news_lg model is slow and orthogonal to what
    these tests cover — entity-linking integration is the unit under test.
    """
    from internal.extractors import entities as entities_module

    class _StubSpan:
        def __init__(self, text: str, label: str, start: int, end: int) -> None:
            self.text = text
            self.label_ = label
            self.start_char = start
            self.end_char = end

    class _StubDoc:
        def __init__(self, ents: Iterable[_StubSpan]) -> None:
            self.ents = list(ents)

    class _StubNlp:
        def __init__(self, ents: Iterable[_StubSpan]) -> None:
            self._ents = list(ents)

        def __call__(self, _text: str) -> _StubDoc:
            return _StubDoc(self._ents)

    extractor = entities_module.NamedEntityExtractor.__new__(
        entities_module.NamedEntityExtractor
    )
    extractor._language_to_model = {"de": "stub", "en": "stub"}
    extractor._default_language = "de"
    extractor._alias_index = fixture_index
    extractor._nlp_by_language = {}
    return extractor, _StubNlp, _StubSpan


def test_extractor_emits_link_for_known_entity(stubbed_extractor) -> None:
    extractor, _StubNlp, _StubSpan = stubbed_extractor
    extractor._nlp_by_language["de"] = _StubNlp(
        [_StubSpan("Angela Merkel", "PER", 0, 13)]
    )

    result = extractor.extract_all(_silver_core("Angela Merkel sagte"), "art-1")
    assert len(result.entities) == 1
    assert len(result.entity_links) == 1
    link = result.entity_links[0]
    assert link.wikidata_qid == "Q567"
    assert link.link_method == "exact_match"
    assert link.entity_text == "Angela Merkel"
    assert link.article_id == "art-1"


def test_extractor_skips_unlinked_spans(stubbed_extractor) -> None:
    extractor, _StubNlp, _StubSpan = stubbed_extractor
    extractor._nlp_by_language["de"] = _StubNlp(
        [
            _StubSpan("Angela Merkel", "PER", 0, 13),
            _StubSpan("Definitely Not A Real Entity", "PER", 20, 50),
        ]
    )

    result = extractor.extract_all(_silver_core("..."), "art-1")
    # Both spans land in entities (canonical SoT), only the linked one in entity_links.
    assert len(result.entities) == 2
    assert len(result.entity_links) == 1
    assert result.entity_links[0].wikidata_qid == "Q567"


def test_extractor_uses_document_language_for_lookup(stubbed_extractor) -> None:
    extractor, _StubNlp, _StubSpan = stubbed_extractor
    extractor._nlp_by_language["en"] = _StubNlp([_StubSpan("Berlin", "LOC", 0, 6)])

    result = extractor.extract_all(_silver_core("Berlin", language="en"), "art-2")
    assert len(result.entity_links) == 1
    assert result.entity_links[0].wikidata_qid == "Q64"


def test_extractor_emits_no_links_when_index_absent(stubbed_extractor) -> None:
    extractor, _StubNlp, _StubSpan = stubbed_extractor
    extractor._alias_index = None
    extractor._nlp_by_language["de"] = _StubNlp(
        [_StubSpan("Angela Merkel", "PER", 0, 13)]
    )

    result = extractor.extract_all(_silver_core("Angela Merkel"), "art-3")
    assert len(result.entities) == 1
    assert result.entity_links == []


def test_repeat_extraction_is_idempotent(stubbed_extractor) -> None:
    """Re-running the extractor on the same document yields the same link rows.

    ReplacingMergeTree(ingestion_version) deduplicates on
    (article_id, entity_text). The processor stamps `ingestion_version`
    from the MinIO event time, so the same article re-delivered through
    NATS produces identical rows. This test guards the data-shape
    invariant; the storage-side dedup is exercised in the full-pipeline
    integration tests.
    """
    extractor, _StubNlp, _StubSpan = stubbed_extractor
    extractor._nlp_by_language["de"] = _StubNlp(
        [_StubSpan("Angela Merkel", "PER", 0, 13)]
    )

    first = extractor.extract_all(_silver_core("..."), "art-1")
    second = extractor.extract_all(_silver_core("..."), "art-1")
    assert [
        (link.entity_text, link.wikidata_qid, link.link_method, link.link_confidence)
        for link in first.entity_links
    ] == [
        (link.entity_text, link.wikidata_qid, link.link_method, link.link_confidence)
        for link in second.entity_links
    ]


# ---------------------------------------------------------------------------
# Hash verification — fail-fast guard for index drift
# ---------------------------------------------------------------------------


def test_hash_mismatch_raises(tmp_path: Path) -> None:
    db_path = tmp_path / "wikidata_test.db"
    _build_fixture_index(db_path)

    with pytest.raises(RuntimeError, match="hash mismatch"):
        WikidataAliasIndex(db_path, expected_sha256="0" * 64)


def test_missing_index_raises(tmp_path: Path) -> None:
    with pytest.raises(FileNotFoundError):
        WikidataAliasIndex(tmp_path / "does_not_exist.db")
