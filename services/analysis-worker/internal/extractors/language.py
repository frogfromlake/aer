import structlog
from langdetect import detect_langs, LangDetectException
from langdetect import DetectorFactory
from lingua import LanguageDetectorBuilder

from internal.extractors.base import GoldMetric, GoldLanguageDetection, ExtractionResult

# Fixed seed for deterministic langdetect across runs (probabilistic model).
DetectorFactory.seed = 0

logger = structlog.get_logger()

# Texts shorter than this prefer the lingua-py verdict on disagreement
# (WP-002 §3.4: langdetect degrades on short RSS descriptions).
#
# Operational note: most Probe 0 RSS descriptions fall below this threshold,
# so on disagreement lingua-py is the de-facto decider for Probe 0. langdetect
# still contributes to the consensus when it agrees with lingua-py and is
# preserved in `aer_gold.language_detections` at rank 2..N+1 for audit. The
# threshold becomes load-bearing for longer-text probes (Phase 122+).
_SHORT_TEXT_THRESHOLD = 100


class LanguageDetectionExtractor:
    """
    Two-detector consensus language detection (Phase 116).

    Runs ``langdetect`` and ``lingua-py`` in parallel on each document. The
    consensus winner becomes the document's ``detected_language``:

    * If both detectors agree, the agreed language wins.
    * If they disagree, ``lingua-py`` wins on texts shorter than
      ``_SHORT_TEXT_THRESHOLD`` (lingua-py benchmarks above langdetect on
      short German/English news headlines, Stahl 2024); ``langdetect``
      wins otherwise (preserves historical comparability for long texts).

    Both detectors' raw top picks are persisted in ``aer_gold.language_detections``
    at separate ranks for provenance and downstream audit:

    * ``rank=1``: consensus winner (document's primary detected language).
    * ``rank=2..N+1``: ``langdetect`` full ranked candidates (historical
      comparability — preserves the pre-Phase-116 output shape).
    * ``rank=N+2``: ``lingua-py`` raw top pick.

    The ``language_confidence`` metric value is the consensus winner's
    confidence as reported by the winning detector.

    The ``language_variety`` column on each row is intentionally empty here —
    it is populated by the processor from ``RssMeta.feed_url`` TLD before the
    ClickHouse insert (extractors stay meta-blind, Phase 76).

    **Provisional (Phase 42)** — extractor is part of the proof-of-concept
    NLP pipeline; methodology will be revisited with CSS researchers (§13.5).
    """

    def __init__(self):
        # lingua-py model loading is expensive (~150 MB resident). Build once,
        # reuse for the worker's lifetime. ``with_preloaded_language_models``
        # forces full load at startup so the first document doesn't pay the
        # lazy-load tax.
        self._lingua = (
            LanguageDetectorBuilder
            .from_all_languages()
            .with_preloaded_language_models()
            .build()
        )
        logger.info("lingua-py detector loaded (all languages, preloaded)")

    @property
    def name(self) -> str:
        return "language_detection"

    def _langdetect_candidates(self, text: str):
        """Return langdetect ranked candidates, or [] on failure."""
        try:
            return detect_langs(text)
        except LangDetectException:
            return []

    def _lingua_top(self, text: str) -> tuple[str, float] | None:
        """Return (iso_code, confidence) for lingua-py's top pick, or None."""
        values = self._lingua.compute_language_confidence_values(text)
        if not values:
            return None
        top = values[0]
        iso = top.language.iso_code_639_1.name.lower()
        return (iso, float(top.value))

    def extract_all(self, core, article_id: str | None) -> ExtractionResult:
        text = core.cleaned_text
        if not text or len(text.strip()) < 10:
            return ExtractionResult()

        ld_candidates = self._langdetect_candidates(text)
        lingua_top = self._lingua_top(text)

        if not ld_candidates and lingua_top is None:
            logger.warning(
                "Both language detectors failed",
                source=core.source,
                article_id=article_id,
                text_length=len(text),
            )
            return ExtractionResult()

        # Determine consensus winner.
        ld_top_lang = ld_candidates[0].lang if ld_candidates else None
        ld_top_prob = float(ld_candidates[0].prob) if ld_candidates else 0.0
        lingua_lang = lingua_top[0] if lingua_top else None
        lingua_prob = lingua_top[1] if lingua_top else 0.0

        if ld_top_lang and lingua_lang and ld_top_lang == lingua_lang:
            winner_lang, winner_prob = ld_top_lang, ld_top_prob
        elif ld_top_lang and lingua_lang:
            # Disagreement: short texts → lingua-py, otherwise → langdetect.
            if len(text) < _SHORT_TEXT_THRESHOLD:
                winner_lang, winner_prob = lingua_lang, lingua_prob
            else:
                winner_lang, winner_prob = ld_top_lang, ld_top_prob
        elif ld_top_lang:
            winner_lang, winner_prob = ld_top_lang, ld_top_prob
        else:
            winner_lang, winner_prob = lingua_lang, lingua_prob

        metrics = [
            GoldMetric(
                timestamp=core.timestamp,
                value=round(winner_prob, 4),
                source=core.source,
                metric_name="language_confidence",
                article_id=article_id,
            ),
        ]

        # Persistence layout: rank=1 consensus winner; ranks 2..N+1 langdetect
        # raw candidates (historical compat); final rank lingua-py top pick.
        detections: list[GoldLanguageDetection] = [
            GoldLanguageDetection(
                timestamp=core.timestamp,
                source=core.source,
                article_id=article_id,
                detected_language=winner_lang,
                confidence=round(winner_prob, 4),
                rank=1,
            ),
        ]
        rank_cursor = 2
        for cand in ld_candidates:
            detections.append(
                GoldLanguageDetection(
                    timestamp=core.timestamp,
                    source=core.source,
                    article_id=article_id,
                    detected_language=cand.lang,
                    confidence=round(float(cand.prob), 4),
                    rank=rank_cursor,
                )
            )
            rank_cursor += 1
        if lingua_top is not None:
            detections.append(
                GoldLanguageDetection(
                    timestamp=core.timestamp,
                    source=core.source,
                    article_id=article_id,
                    detected_language=lingua_lang,
                    confidence=round(lingua_prob, 4),
                    rank=rank_cursor,
                )
            )

        return ExtractionResult(metrics=metrics, language_detections=detections)
