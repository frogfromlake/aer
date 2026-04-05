import structlog
import spacy

from internal.extractors.base import GoldMetric, GoldEntity

logger = structlog.get_logger()


class NamedEntityExtractor:
    """
    Named Entity Recognition using spaCy de_core_news_lg.

    **Provisional (Phase 42)** — This is a proof-of-concept, not a
    scientifically validated implementation.

    Produces:
    - metric_name = "entity_count": Total number of entities found in the text.
    - GoldEntity records: Raw entity spans stored in aer_gold.entities.

    The extract() method returns entity_count as a GoldMetric.
    The extract_entities() method returns GoldEntity records for separate
    insertion into aer_gold.entities by the DataProcessor.

    Limitations (to be addressed with interdisciplinary collaboration, §13.5):
    - spaCy NER on RSS feed descriptions (short, truncated summaries) will
      produce different results than NER on full articles. Recall is expected
      to be significantly lower on short texts.
    - Entity linking is NOT implemented — raw entity spans are stored.
      "Merkel", "Angela Merkel", and "Bundeskanzlerin Merkel" are three
      separate entities, not resolved to a canonical identifier.
    - The model version, entity taxonomy (PER, ORG, LOC, MISC), and
      post-processing pipeline will evolve with the research.
    - German compound words and named entities spanning multiple tokens
      are handled by spaCy's statistical model, not by rule-based patterns.
    - The de_core_news_lg model (~500MB) is trained on German news text,
      which is a reasonable domain match for Probe 0 but may not generalize
      to other domains or registers.
    """

    def __init__(self, model_name: str = "de_core_news_lg"):
        try:
            self._nlp = spacy.load(model_name, disable=["tagger", "parser", "lemmatizer"])
            logger.info(
                "spaCy NER model loaded",
                model=model_name,
                model_version=self._nlp.meta.get("version", "unknown"),
            )
        except OSError:
            logger.error(
                "spaCy model not found — NER extraction disabled. "
                "Install via: python -m spacy download %s",
                model_name,
            )
            self._nlp = None

    @property
    def name(self) -> str:
        return "named_entity"

    def _process(self, core):
        """Run spaCy NER pipeline once per document. Returns doc or None."""
        if self._nlp is None:
            return None
        text = core.cleaned_text
        if not text:
            return None
        return self._nlp(text)

    def extract(self, core, article_id: str | None) -> list[GoldMetric]:
        """Returns entity_count as a Gold metric."""
        doc = self._process(core)
        if doc is None:
            return []

        # Cache doc for extract_entities() to avoid double processing.
        self._last_doc = doc
        self._last_core_id = id(core)

        return [
            GoldMetric(
                timestamp=core.timestamp,
                value=float(len(doc.ents)),
                source=core.source,
                metric_name="entity_count",
                article_id=article_id,
            ),
        ]

    def extract_entities(self, core, article_id: str | None) -> list[GoldEntity]:
        """
        Returns structured GoldEntity records for aer_gold.entities.

        Called by the DataProcessor after extract(). Reuses the cached
        spaCy doc from extract() to avoid double processing.
        """
        # Reuse cached doc if extract() was called on the same core object.
        if hasattr(self, "_last_doc") and self._last_core_id == id(core):
            doc = self._last_doc
        else:
            doc = self._process(core)

        if doc is None:
            return []

        return [
            GoldEntity(
                timestamp=core.timestamp,
                source=core.source,
                article_id=article_id,
                entity_text=ent.text,
                entity_label=ent.label_,
                start_char=ent.start_char,
                end_char=ent.end_char,
            )
            for ent in doc.ents
        ]
