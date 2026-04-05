from internal.extractors.base import GoldMetric


class TemporalDistributionExtractor:
    """
    Extracts temporal distribution metrics from SilverCore.timestamp.

    Pure metadata extraction — no NLP involved. Produces two metrics:
    - publication_hour: Hour of day (0–23) in UTC.
    - publication_weekday: Day of week (0=Monday, 6=Sunday), ISO 8601.

    This extractor is methodologically stable and NOT provisional.
    Temporal metadata extraction is deterministic and requires no
    calibration or interdisciplinary validation.
    """

    @property
    def name(self) -> str:
        return "temporal_distribution"

    def extract(self, core, article_id: str | None) -> list[GoldMetric]:
        ts = core.timestamp
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
