import logging

from internal.extractors.base import GoldMetric

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

    def extract(self, core, article_id: str | None) -> list[GoldMetric]:
        ts = core.timestamp
        if ts.tzinfo is None or ts.utcoffset().total_seconds() != 0:
            logger.warning(
                "temporal_distribution: timestamp is not UTC-aware (tzinfo=%r, utcoffset=%r) "
                "for article_id=%r — skipping temporal metrics",
                ts.tzinfo,
                ts.utcoffset(),
                article_id,
            )
            return []
        return [
            GoldMetric(
                timestamp=ts,
                value=float(ts.hour),
                source=core.source,
                metric_name="publication_hour",
                article_id=article_id,
            ),
            GoldMetric(
                timestamp=ts,
                value=float(ts.weekday()),
                source=core.source,
                metric_name="publication_weekday",
                article_id=article_id,
            ),
        ]
