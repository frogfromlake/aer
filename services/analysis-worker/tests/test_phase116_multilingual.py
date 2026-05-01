"""Phase 116 — Multilingual foundation tests.

Five regression cases mandated by ROADMAP Phase 116:
  (a) short German text → consensus agrees on `de`
  (b) German text from `.at` feed URL → `language_variety='de-AT'`
  (c) English text → NER produces zero spans without crash
  (d) English text → SentimentExtractor writes no metric row
  (e) German Probe-0 fixtures → NER entity count within ±5% of pre-routing
      baseline (regression guard — routing logic must not affect the
      German path).
"""

from __future__ import annotations

import json
from datetime import datetime, timezone
from unittest.mock import MagicMock

import pytest

from internal.extractors import (
    LanguageDetectionExtractor,
    NamedEntityExtractor,
    SentimentExtractor,
)
from internal.processor import _derive_language_variety
from internal.adapters.rss import RssMeta
from internal.models import SilverCore
from conftest import _make_processor, gold_insert_calls


# ---------------------------------------------------------------------------
# Shared fixtures
# ---------------------------------------------------------------------------

GERMAN_SHORT = "Die Bundesregierung beschließt das Klimapaket."
GERMAN_LONG = (
    "Die Bundesregierung hat am Mittwoch ein umfassendes Maßnahmenpaket "
    "zum Klimaschutz verabschiedet. Bundeskanzler Olaf Scholz traf sich "
    "in Berlin mit Vertretern der Industrie und der Umweltverbände."
)
ENGLISH_TEXT = (
    "Prime Minister Keir Starmer met with President Biden in Washington "
    "to discuss the upcoming climate summit and bilateral trade relations."
)


def _core(text: str, *, language: str = "und", source: str = "tagesschau") -> SilverCore:
    return SilverCore(
        document_id="abc123",
        source=source,
        source_type="rss",
        raw_text=text,
        cleaned_text=text,
        language=language,
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=len(text.split()),
    )


# ---------------------------------------------------------------------------
# (a) Short German text → consensus agrees on `de`
# ---------------------------------------------------------------------------

def test_consensus_short_german_text_agrees_on_de():
    extractor = LanguageDetectionExtractor()
    result = extractor.extract_all(_core(GERMAN_SHORT), "article-1")

    assert result.language_detections, "expected at least the consensus row"
    primary = next(d for d in result.language_detections if d.rank == 1)
    assert primary.detected_language == "de"

    # langdetect raw top (rank=2) and lingua-py raw top (last rank) should
    # both agree on `de` for an unambiguous German sentence.
    raw_picks = [d.detected_language for d in result.language_detections if d.rank > 1]
    assert raw_picks, "expected provenance rows from both detectors"
    assert all(p == "de" for p in raw_picks)


# ---------------------------------------------------------------------------
# (b) `.at` feed URL → language_variety = 'de-AT'
# ---------------------------------------------------------------------------

@pytest.mark.parametrize(
    "feed_url,expected",
    [
        ("https://www.derstandard.at/rss", "de-AT"),
        ("https://www.srf.ch/news/feed.xml", "de-CH"),
        ("https://www.tagesschau.de/index~rss2.xml", "de-DE"),
        ("", "de-DE"),  # de detected, no feed_url → default to de-DE
    ],
)
def test_derive_language_variety_for_de(feed_url: str, expected: str):
    meta = RssMeta(source_type="rss", feed_url=feed_url)
    assert _derive_language_variety(meta, "de") == expected


def test_derive_language_variety_skips_non_de():
    meta = RssMeta(source_type="rss", feed_url="https://example.com/rss")
    assert _derive_language_variety(meta, "en") == ""


def test_derive_language_variety_handles_missing_meta():
    assert _derive_language_variety(None, "de") == ""


# ---------------------------------------------------------------------------
# (c) English text → NER produces zero spans without crash
# ---------------------------------------------------------------------------

def test_ner_skips_english_text_no_crash():
    extractor = NamedEntityExtractor()
    if not extractor._nlp_by_language:
        pytest.skip("spaCy de_core_news_lg not installed")

    result = extractor.extract_all(_core(ENGLISH_TEXT, language="en"), "en-1")
    # Phase 116 absence-not-wrong guarantee: no metrics, no entity spans,
    # no exception. The German model must not produce phantom entities on
    # English prose.
    assert result.metrics == []
    assert result.entities == []


# ---------------------------------------------------------------------------
# (d) English text → SentimentExtractor writes no metric row
# ---------------------------------------------------------------------------

def test_sentiment_skips_english_text_no_metric():
    extractor = SentimentExtractor()
    if not extractor._lexicon:
        pytest.skip("SentiWS lexicon not bundled")

    result = extractor.extract_all(_core(ENGLISH_TEXT, language="en"), "en-1")
    # Genuine absence — distinguishes "no sentiment" from "zero sentiment"
    # for cross-language corpus statistics.
    assert result.metrics == []


def test_sentiment_runs_on_legacy_und_language():
    """`und` is the adapter default; sentiment must still run for backward compat."""
    extractor = SentimentExtractor()
    if not extractor._lexicon:
        pytest.skip("SentiWS lexicon not bundled")

    result = extractor.extract_all(_core(GERMAN_LONG, language="und"), "und-1")
    assert len(result.metrics) == 1


# ---------------------------------------------------------------------------
# (e) Regression: German path entity count unchanged ±5%
# ---------------------------------------------------------------------------

# Pre-routing baselines were captured by running the un-routed Phase-42
# extractor on these German fixtures and recording the entity count.
# A change here means the routing logic affected the German pipeline
# itself — the regression guard fires.
GERMAN_REGRESSION_FIXTURES: list[tuple[str, int]] = [
    (
        "Bundeskanzler Olaf Scholz traf sich in Berlin mit dem französischen "
        "Präsidenten Emmanuel Macron.",
        4,
    ),
    (
        "Bundespräsident Frank-Walter Steinmeier reiste nach München zum "
        "Gespräch mit Markus Söder.",
        3,
    ),
    (
        "Angela Merkel und Emmanuel Macron trafen sich in Paris.",
        3,
    ),
    (
        "Der Deutsche Bundestag in Berlin debattierte über das neue Gesetz "
        "zur Energiewende, vorgestellt von Robert Habeck.",
        3,
    ),
]


def test_ner_german_path_regression_within_five_percent():
    extractor = NamedEntityExtractor()
    if not extractor._nlp_by_language:
        pytest.skip("spaCy de_core_news_lg not installed")

    for text, baseline in GERMAN_REGRESSION_FIXTURES:
        result = extractor.extract_all(_core(text, language="de"), "art")
        actual = int(result.metrics[0].value) if result.metrics else 0
        # ±5% with a floor of 1 entity tolerance for small counts.
        tolerance = max(1, int(round(baseline * 0.05)))
        assert abs(actual - baseline) <= tolerance, (
            f"NER regression on German path: text={text!r} "
            f"baseline={baseline} actual={actual} tolerance=±{tolerance}"
        )


# ---------------------------------------------------------------------------
# Processor end-to-end: language_variety lands on the ClickHouse insert
# ---------------------------------------------------------------------------

def _bronze_payload(feed_url: str) -> bytes:
    return json.dumps({
        "source": "tagesschau",
        "source_type": "rss",
        "title": "Test",
        "raw_text": GERMAN_LONG,
        "url": "https://example.com/article",
        "timestamp": "2026-04-05T10:00:00Z",
        "feed_url": feed_url,
    }).encode("utf-8")


def test_processor_emits_language_variety_for_at_feed(
    mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span
):
    extractors = [LanguageDetectionExtractor()]
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors)

    response = MagicMock()
    response.read.return_value = _bronze_payload("https://www.derstandard.at/rss")
    mock_minio.get_object.return_value = response
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    proc.process_event(
        "rss/derstandard/abc123/2026-04-05.json",
        "2026-04-05T10:00:00.000Z",
        dummy_span,
    )

    lang_calls = [
        c for c in gold_insert_calls(mock_clickhouse)
        if c[0][0] == "aer_gold.language_detections"
    ]
    assert lang_calls, "expected an insert into aer_gold.language_detections"
    rows = lang_calls[0][0][1]
    column_names = lang_calls[0][1]["column_names"]

    assert "language_variety" in column_names
    variety_idx = column_names.index("language_variety")
    detected_idx = column_names.index("detected_language")
    rank_idx = column_names.index("rank")

    de_rows = [r for r in rows if r[detected_idx] == "de"]
    assert de_rows, "expected at least one detected_language=de row"
    for r in de_rows:
        assert r[variety_idx] == "de-AT"

    # Non-de provenance rows (if any) carry the empty default — variety is
    # only meaningful for German texts in Phase 116.
    for r in rows:
        if r[detected_idx] != "de":
            assert r[variety_idx] == ""

    # Sanity: the consensus winner row exists at rank=1.
    assert any(r[rank_idx] == 1 for r in rows)
