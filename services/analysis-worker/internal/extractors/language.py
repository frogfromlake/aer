import structlog
from langdetect import detect_langs, LangDetectException
from langdetect import DetectorFactory

from internal.extractors.base import GoldMetric, GoldLanguageDetection, ExtractionResult

# Fixed seed for deterministic language detection across runs.
# langdetect uses a probabilistic model internally; without a fixed seed,
# results vary between invocations on the same input.
DetectorFactory.seed = 0

logger = structlog.get_logger()


class LanguageDetectionExtractor:
    """
    Detects the language of SilverCore.cleaned_text using langdetect.

    **Provisional (Phase 42)** — This is a proof-of-concept, not a
    scientifically validated implementation.

    Implements the unified MetricExtractor protocol (Phase 52): a single
    extract_all() call returns both the language_confidence GoldMetric and
    GoldLanguageDetection records in one pass. Stateless between documents.

    Produces:
    - metric_name = "language_confidence": The probability score (0.0-1.0)
      for the most likely detected language.
    - GoldLanguageDetection records: Ranked language candidates stored in
      aer_gold.language_detections (Phase 45, Migration 004).

    Additionally sets SilverCore.language during adapter harmonization
    (see note below).

    Limitations (to be addressed with interdisciplinary collaboration, §13.5):
    - Language detection accuracy degrades significantly on short texts
      (<50 characters). RSS feed descriptions are often short and truncated.
    - langdetect is optimized for longer documents (paragraphs+).
    - The library's language profiles may not cover all relevant languages
      for future AER probes beyond German.
    - A production-grade implementation may require corpus-level language
      profiling, multilingual model stacking, or lingua-py as an alternative.
    - The fixed seed ensures determinism but does not guarantee accuracy.

    Note on SilverCore.language: This extractor does NOT modify SilverCore
    at extraction time (extractors receive immutable SilverCore). Language
    detection results are stored as Gold metrics. The adapter-level
    SilverCore.language field ("de" for RSS, "und" for legacy) remains the
    authoritative language tag for downstream processing until a validated
    language detection pipeline replaces the hardcoded adapter defaults.
    """

    @property
    def name(self) -> str:
        return "language_detection"

    def extract_all(self, core, article_id: str | None) -> ExtractionResult:
        """
        Single-pass extraction returning the language_confidence metric and
        ranked GoldLanguageDetection records for aer_gold.language_detections.
        """
        text = core.cleaned_text
        if not text or len(text.strip()) < 10:
            return ExtractionResult()

        try:
            results = detect_langs(text)
        except LangDetectException:
            logger.warning(
                "Language detection failed",
                source=core.source,
                article_id=article_id,
                text_length=len(text),
            )
            return ExtractionResult()

        if not results:
            return ExtractionResult()

        top = results[0]
        metrics = [
            GoldMetric(
                timestamp=core.timestamp,
                value=round(top.prob, 4),
                source=core.source,
                metric_name="language_confidence",
                article_id=article_id,
            ),
        ]

        language_detections = [
            GoldLanguageDetection(
                timestamp=core.timestamp,
                source=core.source,
                article_id=article_id,
                detected_language=r.lang,
                confidence=round(r.prob, 4),
                rank=idx + 1,
            )
            for idx, r in enumerate(results)
        ]

        return ExtractionResult(metrics=metrics, language_detections=language_detections)
