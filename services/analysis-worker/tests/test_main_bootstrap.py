"""
Bootstrap-level tests for the analysis worker's extractor initialization.

Phase 73 gate: a single failing extractor must never prevent the worker
from starting. The loop in ``main.init_extractors`` is the sole choke
point for graceful degradation of NLP dependencies (SentiWS lexicon,
spaCy model, langdetect, etc.), so it is tested in isolation from the
async bootstrap that depends on NATS/MinIO/ClickHouse.
"""

from pathlib import Path

import pytest

from internal.extractors import (
    GermanNewsBertSentimentExtractor,
    LanguageDetectionExtractor,
    MultilingualBertSentimentExtractor,
    NamedEntityExtractor,
    SentimentExtractor,
    TemporalDistributionExtractor,
    WordCountExtractor,
)
from main import DEFAULT_EXTRACTOR_CLASSES, init_extractors


def test_default_classes_match_expected_pipeline():
    assert DEFAULT_EXTRACTOR_CLASSES == [
        WordCountExtractor,
        TemporalDistributionExtractor,
        LanguageDetectionExtractor,
        SentimentExtractor,
        # Phase 119: Tier-2 default + Tier-2.5 German-news refinement
        # register alongside Tier-1 SentiWS (ADR-023).
        MultilingualBertSentimentExtractor,
        GermanNewsBertSentimentExtractor,
        NamedEntityExtractor,
    ]


def test_init_extractors_skips_failing_class():
    class _Boom:
        def __init__(self):
            raise RuntimeError("dependency missing")

    result = init_extractors([WordCountExtractor, _Boom, TemporalDistributionExtractor])

    assert len(result) == 2
    assert isinstance(result[0], WordCountExtractor)
    assert isinstance(result[1], TemporalDistributionExtractor)


def test_init_extractors_all_fail_returns_empty():
    class _Boom:
        def __init__(self):
            raise ValueError("nope")

    result = init_extractors([_Boom, _Boom])
    assert result == []


def test_worker_starts_without_sentiws_lexicon(tmp_path: Path, monkeypatch):
    """
    Empty SentiWS directory must not crash worker startup. The current
    SentimentExtractor degrades by producing no metrics; a future
    hard-failing variant must still be caught by ``init_extractors``.
    """
    empty_sentiws = tmp_path / "sentiws_empty"
    empty_sentiws.mkdir()

    import internal.extractors.sentiment as sentiment_mod

    monkeypatch.setattr(sentiment_mod, "_DEFAULT_SENTIWS_DIR", empty_sentiws)

    # Include only extractors that do not require heavy external models here;
    # NER/Language/Sentiment are allowed to either succeed or be skipped.
    classes = [WordCountExtractor, TemporalDistributionExtractor, SentimentExtractor]
    extractors = init_extractors(classes)

    # WordCount + Temporal are deterministic and must always initialize.
    names = {type(e).__name__ for e in extractors}
    assert "WordCountExtractor" in names
    assert "TemporalDistributionExtractor" in names

    # Sentiment either initialized with an empty lexicon (current behavior) or
    # was skipped by the init loop — both are valid graceful-degradation paths.
    sentiments = [e for e in extractors if isinstance(e, SentimentExtractor)]
    if sentiments:
        assert sentiments[0]._lexicon == {}


def test_init_extractors_simulated_sentiment_failure():
    """Belt-and-braces: if SentimentExtractor were to raise during init,
    the worker must continue with the remaining pipeline."""

    class _BrokenSentiment(SentimentExtractor):
        def __init__(self):  # noqa: D401
            raise RuntimeError("SentiWS lexicon missing")

    result = init_extractors(
        [WordCountExtractor, TemporalDistributionExtractor, _BrokenSentiment]
    )

    assert len(result) == 2
    assert not any(isinstance(e, SentimentExtractor) for e in result)


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
