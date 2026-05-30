import logging
from datetime import timezone

from internal.extractors.base import GoldMetric, ExtractionResult

logger = logging.getLogger(__name__)


class TemporalDistributionExtractor:
    """
    Extracts temporal distribution metrics from SilverCore.timestamp.

    Pure metadata extraction — no NLP involved. Produces two metrics:
    - publication_hour: Hour of day (0–23) in UTC.
    - publication_weekday: Day of week (0=Monday, 6=Sunday), ISO 8601.

    This extractor is methodologically stable and NOT provisional.
    Temporal metadata extraction is deterministic and requires no
    calibration or interdisciplinary validation.

    Defense-in-depth: validates that the timestamp is timezone-aware and UTC
    before extracting hour/weekday. The Silver contract (SilverCore) enforces
    timezone-awareness at construction time; this guard is an additional safety
    net against non-UTC offsets that would silently produce wrong metrics.
    """

    @property
    def name(self) -> str:
        return "temporal_distribution"

    def extract_all(self, core, article_id: str | None) -> ExtractionResult:
        ts = core.timestamp
        # Only genuinely-naive timestamps are unusable. An aware timestamp with
        # a non-zero offset (e.g. +02:00 CEST on FR/DE sources) is valid — we
        # normalise it to UTC so the hour/weekday axis is uniform across every
        # source and cultural context. Dropping it (the pre-Phase-123 behaviour)
        # silently lost publication_hour/weekday for every non-UTC publisher.
        if ts.tzinfo is None or ts.utcoffset() is None:
            logger.warning(
                "temporal_distribution: timestamp is naive (tzinfo=%r) "
                "for article_id=%r — skipping temporal metrics",
                ts.tzinfo,
                article_id,
            )
            return ExtractionResult()
        ts_utc = ts.astimezone(timezone.utc)
        return ExtractionResult(
            metrics=[
                GoldMetric(
                    timestamp=ts_utc,
                    value=float(ts_utc.hour),
                    source=core.source,
                    metric_name="publication_hour",
                    article_id=article_id,
                ),
                GoldMetric(
                    timestamp=ts_utc,
                    value=float(ts_utc.weekday()),
                    source=core.source,
                    metric_name="publication_weekday",
                    article_id=article_id,
                ),
            ]
        )
