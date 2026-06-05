"""Phase 122d.3 — Silent-Edit Discourse Shift delta unit tests.

Pins the two correctness-critical invariants without loading any model:

1. **Sign convention.** ``orient_pair`` must order a pair earliest→latest,
   and the ``chain_head`` kind is REVERSED in time (its ``prev`` side is the
   current article body, chronologically LATER, and ``curr`` is the older
   snapshot). Getting this wrong inverts every sentiment-delta sign and
   swaps added/removed entities for the ~half of the corpus that is
   single-snapshot — the exact bug class the Phase-133 review caught.

2. **deltas_computed honesty.** A known-language edit yields a full delta
   (``deltas_computed=True``); an unknown-language pair records only the
   language-agnostic topic_shift with ``deltas_computed=False``.

The extractors are faked to keep the test fast and model-free; only the
``extract_all`` contract (``.metrics[].value`` / ``.entities[].entity_text``)
and the ``_nlp_by_language`` routing guard are exercised.
"""

from __future__ import annotations

from dataclasses import dataclass

from internal.extractors.base import ExtractionResult, GoldEntity, GoldMetric
from internal.extractors.revision_deltas import (
    RevisionDeltaTools,
    compute_deltas,
    orient_pair,
)
from datetime import datetime, timezone

_STAMP = datetime(2000, 1, 1, tzinfo=timezone.utc)


class _FakeSentiment:
    """Maps a text to a fixed scalar via a lookup; mimics extract_all."""

    def __init__(self, scores: dict[str, float]):
        self._scores = scores
        self._model_name = "fake-xlmr"
        self._model_revision = "rev0"

    def extract_all(self, core, _article_id):
        if core.cleaned_text not in self._scores:
            return ExtractionResult()
        return ExtractionResult(
            metrics=[
                GoldMetric(
                    timestamp=_STAMP,
                    value=self._scores[core.cleaned_text],
                    source=core.source,
                    metric_name="sentiment_score_bert_multilingual",
                    article_id=None,
                )
            ]
        )


class _FakeNer:
    """Maps a text to a set of entity spans; carries a routing table."""

    def __init__(self, spans: dict[str, list[str]], languages: list[str]):
        self._spans = spans
        self._nlp_by_language = {lang: object() for lang in languages}
        self._language_to_model = {lang: f"{lang}_core_news_lg" for lang in languages}

    def extract_all(self, core, _article_id):
        return ExtractionResult(
            entities=[
                GoldEntity(
                    timestamp=_STAMP,
                    source=core.source,
                    article_id=None,
                    entity_text=span,
                    entity_label="MISC",
                    start_char=0,
                    end_char=len(span),
                )
                for span in self._spans.get(core.cleaned_text, [])
            ]
        )


@dataclass
class _FakeEmbedder:
    distance: float = 0.4

    def cosine_distance(self, _a, _b):
        return self.distance


# --------------------------------------------------------------------------
# orient_pair — the sign-convention authority.
# --------------------------------------------------------------------------


def test_orient_pair_mid_chain_keeps_prev_older():
    older, newer = orient_pair("mid_chain", prev_text="OLD", curr_text="NEW")
    assert (older, newer) == ("OLD", "NEW")


def test_orient_pair_chain_head_is_reversed():
    # chain_head: prev=current body (LATER), curr=newest snapshot (EARLIER).
    older, newer = orient_pair("chain_head", prev_text="CURRENT", curr_text="SNAPSHOT")
    assert (older, newer) == ("SNAPSHOT", "CURRENT")


# --------------------------------------------------------------------------
# compute_deltas — later-minus-earlier, end to end through orient_pair.
# --------------------------------------------------------------------------


def test_chain_head_sign_current_more_positive_gives_positive_delta():
    """The article got more positive since the snapshot ⇒ sentiment_delta > 0,
    and an entity only in the current article is ADDED (not removed)."""
    snapshot, current = "SNAPSHOT", "CURRENT"
    tools = RevisionDeltaTools(
        sentiment=_FakeSentiment({snapshot: -0.5, current: 0.6}),
        ner=_FakeNer({snapshot: ["Merkel"], current: ["Merkel", "Scholz"]}, ["de"]),
        embedder=_FakeEmbedder(0.3),
    )
    older, newer = orient_pair("chain_head", prev_text=current, curr_text=snapshot)
    result = compute_deltas(older, newer, "de", tools)

    assert result.sentiment_delta == 1.1  # 0.6 - (-0.5)
    assert result.entities_added == ["Scholz"]
    assert result.entities_removed == []
    assert result.topic_shift_score == 0.3
    assert result.deltas_computed is True


def test_mid_chain_sign_and_entity_removal():
    older, newer = orient_pair("mid_chain", prev_text="OLD", curr_text="NEW")
    tools = RevisionDeltaTools(
        sentiment=_FakeSentiment({"OLD": 0.4, "NEW": 0.1}),
        ner=_FakeNer({"OLD": ["Putin", "Selenskyj"], "NEW": ["Putin"]}, ["de"]),
        embedder=_FakeEmbedder(0.5),
    )
    result = compute_deltas(older, newer, "de", tools)

    assert round(result.sentiment_delta, 4) == -0.3  # 0.1 - 0.4
    assert result.entities_added == []
    assert result.entities_removed == ["Selenskyj"]
    assert result.deltas_computed is True


def test_unknown_language_records_topic_shift_only():
    tools = RevisionDeltaTools(
        sentiment=_FakeSentiment({"A": 0.2, "B": -0.2}),
        ner=_FakeNer({"A": ["X"], "B": ["Y"]}, ["de"]),
        embedder=_FakeEmbedder(0.7),
    )
    result = compute_deltas("A", "B", "und", tools)

    assert result.deltas_computed is False
    assert result.sentiment_delta == 0.0
    assert result.entities_added == []
    assert result.entities_removed == []
    assert result.topic_shift_score == 0.7  # language-agnostic, still recorded


def test_unrouteable_ner_language_skips_entities_but_keeps_sentiment_partial():
    """Sentiment is multilingual (still scores) but NER has no model for the
    language ⇒ entities skipped and the row is marked partial (not full)."""
    tools = RevisionDeltaTools(
        sentiment=_FakeSentiment({"A": 0.2, "B": 0.5}),
        ner=_FakeNer({"A": ["X"], "B": ["Y"]}, ["de"]),  # only 'de' loaded
        embedder=_FakeEmbedder(0.2),
    )
    result = compute_deltas("A", "B", "fr", tools)

    assert result.entities_added == []
    assert result.entities_removed == []
    assert result.deltas_computed is False  # ner not routable for 'fr'


def test_missing_embedder_yields_zero_topic_shift():
    tools = RevisionDeltaTools(
        sentiment=_FakeSentiment({"A": 0.0, "B": 0.0}),
        ner=_FakeNer({"A": [], "B": []}, ["de"]),
        embedder=None,
    )
    result = compute_deltas("A", "B", "de", tools)

    assert result.topic_shift_score == 0.0
    assert result.deltas_computed is True  # sentiment + ner both ran
