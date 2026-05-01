"""Phase 117 — Sentiment Tier 1 hardening tests.

Six regression cases mandated by ROADMAP Phase 117:
  (a) `Das ist nicht gut.`               → negation inverts the positive `gut`
  (b) Embedded clause: `Ich glaube nicht, dass das Projekt gut ist.`
                                          → negation scope clamped at the
                                            subordinator (the embedded `gut`
                                            is *not* inverted)
  (c) German compound `Wutausbruch`      → `Wut` + `Ausbruch` averaged
  (d) Unrecognised compound              → score = 0, no crash
  (e) Custom-lexicon entry overrides SentiWS for that token
  (f) Existing un-negated, non-compound fixture scores within ±5% of the
      Phase-42 baseline (regression guard — the dependency walk must not
      drift the score on bag-of-words sentences).
"""

from __future__ import annotations

from datetime import datetime, timezone
from pathlib import Path

import pytest

from internal.extractors import SentimentExtractor
from internal.models import SilverCore


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _core(text: str, *, language: str = "de") -> SilverCore:
    return SilverCore(
        document_id="abc123",
        source="tagesschau",
        source_type="rss",
        raw_text=text,
        cleaned_text=text,
        language=language,
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=len(text.split()),
    )


def _write_minimal_lexicon(tmp_path: Path) -> Path:
    """A tiny SentiWS-shaped lexicon covering the words used in Phase 117 tests."""
    pos = tmp_path / "SentiWS_v2.0_Positive.txt"
    neg = tmp_path / "SentiWS_v2.0_Negative.txt"
    pos.write_text(
        "gut|ADJX\t0.5040\tguten,guter,gutes\n"
        "Glück|NN\t0.5765\tGlücks\n"
        "Ausbruch|NN\t0.2000\tAusbrüche\n",
        encoding="utf-8",
    )
    neg.write_text(
        "schlecht|ADJX\t-0.4771\tschlechter,schlechten\n"
        "Wut|NN\t-0.6000\tWuten\n",
        encoding="utf-8",
    )
    return tmp_path


def _empty_custom(tmp_path: Path) -> Path:
    p = tmp_path / "custom_lexicon.yaml"
    p.write_text("{}\n", encoding="utf-8")
    return p


# ---------------------------------------------------------------------------
# (a) Sentence-level negation
# ---------------------------------------------------------------------------

def test_negation_inverts_positive_token(tmp_path):
    extractor = SentimentExtractor(
        sentiws_dir=_write_minimal_lexicon(tmp_path),
        custom_lexicon_path=_empty_custom(tmp_path),
    )
    if extractor._nlp is None:
        pytest.skip("spaCy de_core_news_lg not available — negation walk requires the parser")

    pos = extractor.extract_all(_core("Das ist gut."), "p1").metrics
    neg = extractor.extract_all(_core("Das ist nicht gut."), "p2").metrics

    assert pos and neg
    assert pos[0].value > 0
    # Phase 117 fix: negation flips polarity instead of leaving the positive
    # `gut` untouched (the Phase-42 bag-of-words behaviour).
    assert neg[0].value < 0


# ---------------------------------------------------------------------------
# (b) Embedded-clause scope clamp
# ---------------------------------------------------------------------------

def test_negation_scope_does_not_leak_into_embedded_clause(tmp_path):
    extractor = SentimentExtractor(
        sentiws_dir=_write_minimal_lexicon(tmp_path),
        custom_lexicon_path=_empty_custom(tmp_path),
    )
    if extractor._nlp is None:
        pytest.skip("spaCy de_core_news_lg not available")

    # Matrix-clause `nicht` should NOT punish the embedded `gut`. With the
    # clause-boundary clamp the embedded subtree (`dass ...`) is excluded
    # from the inversion set, so the score is dominated by the embedded
    # `gut` and stays positive.
    text = "Ich glaube nicht, dass das Projekt gut ist."
    out = extractor.extract_all(_core(text), "p3").metrics
    assert out
    assert out[0].value > 0


# ---------------------------------------------------------------------------
# (c) Compound decomposition success
# ---------------------------------------------------------------------------

def test_compound_split_scores_known_compound(tmp_path):
    extractor = SentimentExtractor(
        sentiws_dir=_write_minimal_lexicon(tmp_path),
        custom_lexicon_path=_empty_custom(tmp_path),
    )

    # `Wutausbruch` is not in the lexicon directly. The `compound-split`
    # head/tail decomposition (`Wut` + `Ausbruch`) is — averaged polarity
    # is mean(-0.6, 0.2) = -0.2 for that token.
    out = extractor.extract_all(_core("Der Wutausbruch."), "p4").metrics
    assert out
    # Score is the mean of all tokens that resolved (here just one);
    # negative per the head's polarity dominance.
    assert out[0].value < 0


# ---------------------------------------------------------------------------
# (d) Compound failure path
# ---------------------------------------------------------------------------

def test_unrecognised_compound_returns_zero(tmp_path):
    extractor = SentimentExtractor(
        sentiws_dir=_write_minimal_lexicon(tmp_path),
        custom_lexicon_path=_empty_custom(tmp_path),
    )

    # No constituent of `Donaudampfschifffahrtsgesellschaft` is in the
    # minimal lexicon → no token scored → zero, no exception.
    out = extractor.extract_all(_core("Donaudampfschifffahrtsgesellschaft."), "p5").metrics
    assert out
    assert out[0].value == 0.0


# ---------------------------------------------------------------------------
# (e) Custom lexicon override
# ---------------------------------------------------------------------------

def test_custom_lexicon_overrides_sentiws(tmp_path):
    custom = tmp_path / "custom_lexicon.yaml"
    # `gut` is positive in SentiWS (+0.504) — override it to strongly negative.
    custom.write_text("gut: -0.9\n", encoding="utf-8")

    extractor = SentimentExtractor(
        sentiws_dir=_write_minimal_lexicon(tmp_path),
        custom_lexicon_path=custom,
    )

    # Test the override directly rather than going through the parser, so
    # the assertion holds whether or not the spaCy model is installed.
    assert extractor._lexicon["gut"] == pytest.approx(-0.9)


# ---------------------------------------------------------------------------
# (f) Regression guard: un-negated bag-of-words baseline
# ---------------------------------------------------------------------------

def test_unnegated_score_within_5pct_of_phase42_baseline(tmp_path):
    extractor = SentimentExtractor(
        sentiws_dir=_write_minimal_lexicon(tmp_path),
        custom_lexicon_path=_empty_custom(tmp_path),
    )

    # Phase 42 produced mean(0.5040, 0.5765) = 0.54025 for this text.
    # Phase 117 with no negation cues + no compound substitution must
    # land within ±5% of that value, regardless of whether the parser
    # path or the bag-of-words fallback is used.
    expected = (0.5040 + 0.5765) / 2.0
    out = extractor.extract_all(_core("Das ist gut und bringt Glück."), "p6").metrics
    assert out
    delta = abs(out[0].value - expected) / expected
    assert delta <= 0.05, f"score drift {delta:.3%} > 5% (got {out[0].value}, expected {expected})"


# ---------------------------------------------------------------------------
# Metric-name rename guard
# ---------------------------------------------------------------------------

def test_metric_name_is_sentiws_suffixed(tmp_path):
    extractor = SentimentExtractor(
        sentiws_dir=_write_minimal_lexicon(tmp_path),
        custom_lexicon_path=_empty_custom(tmp_path),
    )
    out = extractor.extract_all(_core("Das ist gut."), "p7").metrics
    assert out
    # Phase 117: ADR-016 dual-metric naming is now lexically explicit.
    assert out[0].metric_name == "sentiment_score_sentiws"
