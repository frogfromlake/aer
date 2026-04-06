import structlog
import spacy

from internal.extractors.base import GoldMetric, GoldEntity

logger = structlog.get_logger()


class NamedEntityExtractor:
    """
    Named Entity Recognition using spaCy de_core_news_lg.

    **Provisional (Phase 42)** — This is a proof-of-concept, not a
    scientifically validated implementation.

    Implements the EntityExtractor protocol (Phase 44): produces both
    GoldMetric (entity_count) and GoldEntity records in a single pass
    via extract_all(). Stateless between documents — no mutable caching.

    Produces:
    - metric_name = "entity_count": Total number of entities found in the text.
    - GoldEntity records: Raw entity spans stored in aer_gold.entities.

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

    def extract_all(self, core, article_id: str | None) -> tuple[list[GoldMetric], list[GoldEntity]]:
        """
        Single-pass extraction returning both entity_count metric and
        GoldEntity records. Processes the spaCy doc exactly once.
        """
        doc = self._process(core)
        if doc is None:
            return [], []

        metrics = [
            GoldMetric(
                timestamp=core.timestamp,
                value=float(len(doc.ents)),
                source=core.source,
                metric_name="entity_count",
                article_id=article_id,
            ),
        ]

        entities = [
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

        return metrics, entities

    def extract(self, core, article_id: str | None) -> list[GoldMetric]:
        """Returns entity_count as a Gold metric."""
        metrics, _ = self.extract_all(core, article_id)
        return metrics

    def extract_entities(self, core, article_id: str | None) -> list[GoldEntity]:
        """Returns structured GoldEntity records for aer_gold.entities."""
        _, entities = self.extract_all(core, article_id)
        return entities
