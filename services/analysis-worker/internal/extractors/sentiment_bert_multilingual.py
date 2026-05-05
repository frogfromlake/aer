"""Multilingual BERT sentiment — Tier 2 default per ADR-023 (Phase 119).

Implements the unified ``MetricExtractor`` protocol. Reads its model name,
pinned revision, and the supported-language guard from the Capability
Manifest's ``shared.multilingual_bert`` block — adding a language to the
guard is a manifest YAML edit, not an extractor patch.

Output: scalar ``sentiment_score_bert_multilingual`` in [-1.0, 1.0].
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
    MultilingualBertCapability,
    load_manifest,
)

logger = structlog.get_logger()


class MultilingualBertSentimentExtractor:
    """Tier-2 default sentiment for every language listed in the manifest's
    ``shared.multilingual_bert.supported_languages``.

    Provisional (Phase 119) — the multilingual model is the SOTA at
    implementation time; results remain ``unvalidated`` per ADR-016 until
    the WP-002 annotation study upgrades the row in
    ``aer_gold.metric_validity``.
    """

    METRIC_NAME = "sentiment_score_bert_multilingual"

    def __init__(
        self,
        manifest: CapabilityManifest | None = None,
        *,
        tokenizer=None,
        model=None,
    ):
        self._manifest = manifest if manifest is not None else load_manifest()
        cfg: MultilingualBertCapability | None = self._manifest.shared.multilingual_bert

        if cfg is None:
            self._enabled = False
            self._model_name = ""
            self._model_revision = ""
            # Empty guard set — `extract_all` short-circuits on `not _enabled`
            # before checking it, but we keep the attribute populated so
            # tests can inspect it without special-casing the absent case.
            self._supported_languages: frozenset[str] = frozenset()
            self._tokenizer = None
            self._model = None
            logger.warning(
                "Manifest has no shared.multilingual_bert block — "
                "MultilingualBertSentimentExtractor disabled",
            )
            return

        self._model_name = cfg.model
        self._model_revision = cfg.model_revision
        # Legacy `und` / empty tags fall through so pre-detection documents
        # still get scored — mirrors the SentiWS extractor's language guard.
        self._supported_languages = frozenset(cfg.supported_languages) | {"und", ""}

        # Tests inject pre-loaded tokenizer/model pairs; production loads
        # from the pinned HuggingFace revision.
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
                "Multilingual BERT sentiment extractor ready",
                model=self._model_name,
                revision=self._model_revision,
                supported_languages=sorted(cfg.supported_languages),
            )
        elif not BERT_AVAILABLE:
            logger.warning(
                "transformers/torch not installed — Tier-2 multilingual "
                "sentiment disabled (graceful degradation)"
            )

    @property
    def name(self) -> str:
        return "sentiment_bert_multilingual"

    @property
    def version_hash(self) -> str:
        return _runtime_version_hash(self._model_name, self._model_revision)

    def extract_all(self, core, article_id: str | None) -> ExtractionResult:
        if not self._enabled:
            return ExtractionResult()

        text = core.cleaned_text
        if not text:
            return ExtractionResult()

        lang = (core.language or "").lower()
        if lang not in self._supported_languages:
            return ExtractionResult()

        try:
            score = score_three_class_to_scalar(self._model, self._tokenizer, text)
        except Exception as exc:
            logger.warning(
                "Multilingual BERT inference failed; skipping document",
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
