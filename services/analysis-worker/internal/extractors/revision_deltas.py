"""
Silent-Edit Discourse Shift — re-extraction deltas (Phase 122d.3).

The Phase-122d.1 revision-diff sweep (``corpus.run_revision_diff_sweep``)
already fetches both snapshot HTMLs from the Internet Archive and extracts
their text with trafilatura, purely to compute the paragraph diff. This
module piggybacks on that — given the two snapshot texts already in hand,
it re-extracts sentiment and named entities over each version and computes
the **discourse shift** an edit produced:

  * ``sentiment_delta``   — change in the multilingual-backbone sentiment
    scalar (cross-probe-comparable; ``cardiffnlp/twitter-xlm-roberta-base
    -sentiment``).
  * ``entities_added`` / ``entities_removed`` — set difference of NER
    surface spans between the two versions.
  * ``topic_shift_score`` — cosine distance of the two versions'
    ``intfloat/multilingual-e5-large`` embeddings. Honestly: a SEMANTIC
    shift, not a topic-label switch (BERTopic is a corpus-level fit with
    no per-snapshot label — see ROADMAP Phase 122d.3 grounding).

**Sign convention.** Every delta is computed *later-in-time minus
earlier-in-time*. The sweep is responsible for orienting the pair (its
``chain_head`` pairs are reversed in time — see ``corpus.py``); this module
trusts ``older_text`` / ``newer_text`` as already chronologically ordered.

**Graceful degradation.** No second Wayback fetch, no DLQ. If the E5
embedder cannot load, ``topic_shift_score`` is 0. If the article's language
is unknown (archived-only past the Silver TTL) the language-routed
extractors (sentiment, NER) would mis-route, so they are skipped and
``deltas_computed`` is ``False`` — only the language-agnostic
``topic_shift_score`` is still recorded. ``deltas_computed`` is the honest
marker the BFF aggregations filter on so identical re-archivals and
partial rows never pollute trajectory averages.

The module is IO-free and unit-testable: ``compute_deltas`` takes plain
text + a tool bundle and returns a dataclass.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timezone

import structlog

from internal.extractors.topic_modeling import (
    _DEFAULT_EMBEDDING_MODEL,
    _DEFAULT_EMBEDDING_REVISION,
    _E5_PASSAGE_PREFIX,
)

logger = structlog.get_logger()

# A fixed, timezone-aware stamp for the throwaway SilverCore handed to the
# language-routed extractors. Sentiment/NER read only ``cleaned_text`` +
# ``language`` + ``source`` — the timestamp never reaches their output, so
# a constant keeps re-extraction deterministic.
_REEXTRACTION_STAMP = datetime(2000, 1, 1, tzinfo=timezone.utc)


@dataclass(frozen=True, slots=True)
class DeltaResult:
    """The four discourse-shift deltas for one oriented snapshot pair."""

    sentiment_delta: float = 0.0
    entities_added: list[str] = field(default_factory=list)
    entities_removed: list[str] = field(default_factory=list)
    topic_shift_score: float = 0.0
    # True only when the FULL language-routed pass (sentiment + NER) ran on
    # both sides. Partial rows (language unknown) and identical pairs read
    # False; the BFF filters delta aggregates on it.
    deltas_computed: bool = False


class E5PairEmbedder:
    """Load-once E5 embedder for pairwise cosine distance.

    Reuses the same ``shared.topic_modeling`` model + revision the
    Language Capability Manifest pins for BERTopic, so ``topic_shift_score``
    sits on the identical backbone as Episteme topic discovery. The
    ``SentenceTransformer`` is constructed once at worker boot and reused
    across every sweep tick — NOT per pair.

    ``sentence_transformers`` is late-imported so the worker boots without
    it (the constructor raises; ``main.py`` catches and disables the topic
    delta gracefully).
    """

    PASSAGE_PREFIX = _E5_PASSAGE_PREFIX  # mirror the topic extractor.

    def __init__(
        self,
        *,
        model: str | None = None,
        revision: str | None = None,
        embedder=None,
    ) -> None:
        self.model = model or _DEFAULT_EMBEDDING_MODEL
        self.revision = revision or _DEFAULT_EMBEDDING_REVISION
        if embedder is not None:
            # Test injection — any object exposing ``encode(list, ...)``.
            self._embedder = embedder
            return
        from sentence_transformers import SentenceTransformer  # type: ignore[import-not-found]

        self._embedder = SentenceTransformer(self.model, revision=self.revision)
        logger.info(
            "revision_delta.e5_embedder.loaded",
            model=self.model,
            revision=self.revision,
        )

    def cosine_distance(self, text_a: str, text_b: str) -> float:
        """Cosine distance in [0, 2]; 0 = identical semantic vector.

        E5 is L2-normalised (``normalize_embeddings=True``), so cosine
        similarity is the dot product and distance is ``1 - sim``. The two
        texts are encoded in a single batched ``encode`` call.
        """
        if not text_a or not text_b:
            return 0.0
        embs = self._embedder.encode(
            [self.PASSAGE_PREFIX + text_a, self.PASSAGE_PREFIX + text_b],
            normalize_embeddings=True,
            show_progress_bar=False,
        )
        sim = float(sum(a * b for a, b in zip(embs[0], embs[1])))
        return round(1.0 - sim, 6)


@dataclass(frozen=True, slots=True)
class RevisionDeltaTools:
    """The load-once extractor bundle threaded into the diff sweep.

    ``sentiment`` and ``ner`` reuse the instances the document pipeline
    already loaded (no second model load); ``embedder`` is the only
    genuinely new load. ``delta_tools=None`` in the sweep disables the
    whole delta path (back-compat / models-missing).
    """

    sentiment: object | None = None  # MultilingualBertSentimentExtractor
    ner: object | None = None  # NamedEntityExtractor
    embedder: E5PairEmbedder | None = None


def orient_pair(kind: str, prev_text: str, curr_text: str) -> tuple[str, str]:
    """Order a sweep pair chronologically: ``(older_text, newer_text)``.

    This is the single sign-convention authority for the discourse-shift
    deltas — getting it wrong inverts every "sentiment got more positive"
    reading and swaps added/removed entities for the ~half of the corpus
    that is single-snapshot (``chain_head``). Pinned by unit test.

    * ``mid_chain`` — ``prev`` is the older snapshot, ``curr`` the newer.
    * ``chain_head`` — ``prev`` is the CURRENT Silver body (the article
      now, i.e. chronologically LATER) and ``curr`` is the newest Wayback
      snapshot (EARLIER). Reversed, so the caller still gets earliest→latest.
    """
    if kind == "chain_head":
        return curr_text, prev_text
    return prev_text, curr_text


def _build_core(text: str, language: str, source: str):
    """Minimal SilverCore for re-extraction over a snapshot version.

    Late-imports the pydantic model so this module is importable without
    the full Silver stack in lightweight unit tests that inject fakes.
    """
    from internal.models import SilverCore

    return SilverCore(
        document_id="revision-reextract",
        source=source or "unknown",
        source_type="web",
        raw_text=text,
        cleaned_text=text,
        language=language,
        timestamp=_REEXTRACTION_STAMP,
    )


def _sentiment_scalar(extractor, text: str, language: str, source: str) -> float | None:
    """Run the sentiment extractor over one snapshot version; scalar or None."""
    if extractor is None or not text:
        return None
    result = extractor.extract_all(_build_core(text, language, source), None)
    if not result.metrics:
        return None
    return float(result.metrics[0].value)


def _entity_spans(extractor, text: str, language: str, source: str) -> set[str]:
    """NER surface spans for one snapshot version (raw surface form)."""
    if extractor is None or not text:
        return set()
    result = extractor.extract_all(_build_core(text, language, source), None)
    return {e.entity_text for e in result.entities}


def _language_routable(ner_extractor, language: str) -> bool:
    """Whether NER has a model actually loaded for this language.

    Guards against silently mis-routing a French / English snapshot to the
    German default model (entities.py falls back to ``de`` for unknown
    languages, which would produce a garbage entity set).
    """
    loaded = getattr(ner_extractor, "_nlp_by_language", None)
    if not isinstance(loaded, dict):
        return False
    return language in loaded


def compute_deltas(
    older_text: str,
    newer_text: str,
    language: str | None,
    tools: RevisionDeltaTools,
) -> DeltaResult:
    """Discourse-shift deltas for one chronologically-ordered snapshot pair.

    ``older_text`` / ``newer_text`` are already cleaned text (trafilatura
    output) and already oriented earliest→latest by the caller. Every
    delta is ``f(newer) - f(older)`` so the sign is always
    later-minus-earlier.
    """
    # topic_shift is language-agnostic (E5 is multilingual) — compute it
    # even when the language is unknown.
    topic_shift = 0.0
    if tools.embedder is not None:
        try:
            topic_shift = tools.embedder.cosine_distance(older_text, newer_text)
        except Exception as exc:
            logger.warning(
                "revision_delta.topic_shift.failed",
                error=str(exc),
                error_type=type(exc).__name__,
            )

    lang = (language or "").strip().lower()
    if not lang or lang == "und":
        # Language unknown → the language-routed extractors would mis-route.
        # Record topic_shift only; mark the row partial.
        return DeltaResult(topic_shift_score=topic_shift, deltas_computed=False)

    source = ""  # source does not affect sentiment/NER output values.

    sent_older = _sentiment_scalar(tools.sentiment, older_text, lang, source)
    sent_newer = _sentiment_scalar(tools.sentiment, newer_text, lang, source)
    sentiment_ok = sent_older is not None and sent_newer is not None
    sentiment_delta = round(sent_newer - sent_older, 4) if sentiment_ok else 0.0

    ner_ok = tools.ner is not None and _language_routable(tools.ner, lang)
    if ner_ok:
        spans_older = _entity_spans(tools.ner, older_text, lang, source)
        spans_newer = _entity_spans(tools.ner, newer_text, lang, source)
        added = sorted(spans_newer - spans_older)
        removed = sorted(spans_older - spans_newer)
    else:
        added, removed = [], []

    return DeltaResult(
        sentiment_delta=sentiment_delta,
        entities_added=added,
        entities_removed=removed,
        topic_shift_score=topic_shift,
        deltas_computed=sentiment_ok and ner_ok,
    )
