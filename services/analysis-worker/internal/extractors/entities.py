import structlog
import spacy

from internal.extractors.base import GoldMetric, GoldEntity, ExtractionResult

logger = structlog.get_logger()


class NamedEntityExtractor:
    """
    Named Entity Recognition using spaCy de_core_news_lg.

    **Provisional (Phase 42)** — This is a proof-of-concept, not a
    scientifically validated implementation.

    Implements the unified MetricExtractor protocol (Phase 52): a single
    extract_all() call returns both entity_count GoldMetric and GoldEntity
    records in one pass. Stateless between documents — no mutable caching.

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

    # Named entities are at most a short noun phrase. Anything longer is a
    # spaCy false-positive — typically a sentence fragment labeled MISC when
    # the German model encounters non-German text.
    _MAX_ENTITY_CHARS = 80
    _MAX_ENTITY_WORDS = 6

    # Language codes the German model can handle reliably.
    # "und" (undetermined) is allowed so we don't skip articles whose
    # language detection has not yet run or produced a low-confidence result.
    _SUPPORTED_LANGUAGES = {"de", "und"}

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
        lang = (core.language or "und").lower()
        if lang not in self._SUPPORTED_LANGUAGES:
            logger.debug(
                "Skipping NER — language not supported by de_core_news_lg",
                language=lang,
                source=core.source,
            )
            return None
        return self._nlp(text)

    @classmethod
    def _is_valid_entity(cls, text: str) -> bool:
        """Return False for spans that are clearly not named entities."""
        stripped = text.strip()
        if not stripped:
            return False
        if len(stripped) > cls._MAX_ENTITY_CHARS:
            return False
        if len(stripped.split()) > cls._MAX_ENTITY_WORDS:
            return False
        return True

    def extract_all(self, core, article_id: str | None) -> ExtractionResult:
        """
        Single-pass extraction returning entity_count metric and GoldEntity
        records. Processes the spaCy doc exactly once.
        """
        doc = self._process(core)
        if doc is None:
            return ExtractionResult()

        valid_ents = [ent for ent in doc.ents if self._is_valid_entity(ent.text)]

        metrics = [
            GoldMetric(
                timestamp=core.timestamp,
                value=float(len(valid_ents)),
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
            for ent in valid_ents
        ]

        return ExtractionResult(metrics=metrics, entities=entities)
