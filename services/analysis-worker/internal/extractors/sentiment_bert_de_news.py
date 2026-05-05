"""German news-domain BERT sentiment — Tier 2.5 refinement per ADR-023 (Phase 119).

Activates only when the manifest declares ``languages.de.sentiment_tier2_refinement``
*and* the document's detected language is German. The gap between this
extractor's output and the multilingual default is the WP-002 §3.2
domain-transfer signal that motivated ADR-023's tier split.

Output: scalar ``sentiment_score_bert_de_news`` in [-1.0, 1.0].
"""

from __future__ import annotations

import structlog

from internal.extractors._bert_runtime import (
    BERT_AVAILABLE,
    load_model_and_tokenizer,
    score_three_class_to_scalar,
    version_hash as _runtime_version_hash,
)
from internal.extractors.base import ExtractionResult, GoldMetric
from internal.models.language_capability import (
    CapabilityManifest,
    SentimentTier2RefinementCapability,
    load_manifest,
)

logger = structlog.get_logger()


_TARGET_LANGUAGE = "de"


class GermanNewsBertSentimentExtractor:
    """Tier-2.5 German news-domain sentiment refinement.

    Hard-pinned to ``mdraw/german-news-sentiment-bert`` via the manifest.
    The class is intentionally narrow — German-only, news-domain — so that
    a future French or Italian refinement is its own extractor file with
    its own manifest entry, not a generalisation of this one.
    """

    METRIC_NAME = "sentiment_score_bert_de_news"

    def __init__(
        self,
        manifest: CapabilityManifest | None = None,
        *,
        tokenizer=None,
        model=None,
    ):
        self._manifest = manifest if manifest is not None else load_manifest()
        de_block = self._manifest.languages.get(_TARGET_LANGUAGE)
        cfg: SentimentTier2RefinementCapability | None = (
            de_block.sentiment_tier2_refinement if de_block is not None else None
        )

        if cfg is None:
            self._enabled = False
            self._model_name = ""
            self._model_revision = ""
            self._tokenizer = None
            self._model = None
            logger.info(
                "Manifest has no de.sentiment_tier2_refinement block — "
                "GermanNewsBertSentimentExtractor disabled",
            )
            return

        self._model_name = cfg.model
        self._model_revision = cfg.model_revision

        if tokenizer is not None and model is not None:
            self._tokenizer = tokenizer
            self._model = model
        else:
            self._tokenizer, self._model = load_model_and_tokenizer(
                self._model_name, self._model_revision
            )

        self._enabled = self._tokenizer is not None and self._model is not None
        if self._enabled:
            logger.info(
                "German news BERT sentiment extractor ready",
                model=self._model_name,
                revision=self._model_revision,
            )
        elif not BERT_AVAILABLE:
            logger.warning(
                "transformers/torch not installed — Tier-2.5 German news "
                "sentiment disabled (graceful degradation)"
            )

    @property
    def name(self) -> str:
        return "sentiment_bert_de_news"

    @property
    def version_hash(self) -> str:
        return _runtime_version_hash(self._model_name, self._model_revision)

    def extract_all(self, core, article_id: str | None) -> ExtractionResult:
        if not self._enabled:
            return ExtractionResult()

        text = core.cleaned_text
        if not text:
            return ExtractionResult()

        # Strict language match — Tier-2.5 is the German-only refinement;
        # legacy `und` / empty tags do *not* fall through here because the
        # whole point of the refinement is in-domain quality. A document
        # without a confirmed language gets the Tier-2 multilingual model
        # via the sibling extractor.
        if (core.language or "").lower() != _TARGET_LANGUAGE:
            return ExtractionResult()

        try:
            score = score_three_class_to_scalar(self._model, self._tokenizer, text)
        except Exception as exc:
            logger.warning(
                "German news BERT inference failed; skipping document",
                article_id=article_id,
                error=str(exc),
                error_type=type(exc).__name__,
            )
            return ExtractionResult()

        return ExtractionResult(
            metrics=[
                GoldMetric(
                    timestamp=core.timestamp,
                    value=round(score, 4),
                    source=core.source,
                    metric_name=self.METRIC_NAME,
                    article_id=article_id,
                ),
            ]
        )
