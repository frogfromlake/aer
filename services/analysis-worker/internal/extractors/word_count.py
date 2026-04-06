from internal.extractors.base import GoldMetric, ExtractionResult


class WordCountExtractor:
    """
    Extracts word count from SilverCore.cleaned_text.

    This is the first MetricExtractor implementation, migrated from the
    hardcoded step in DataProcessor (Phase 41). The word count is derived
    from the already whitespace-normalized cleaned_text field.
    """

    @property
    def name(self) -> str:
        return "word_count"

    def extract_all(self, core, article_id: str | None) -> ExtractionResult:
        return ExtractionResult(
            metrics=[
                GoldMetric(
                    timestamp=core.timestamp,
                    value=float(core.word_count),
                    source=core.source,
                    metric_name="word_count",
                    article_id=article_id,
                )
            ]
        )
