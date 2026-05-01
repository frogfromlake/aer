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

    # Phase 116: language-routing map. New languages are added by extending
    # this dict and adding the corresponding spaCy model to requirements.txt
    # (e.g. `fr: fr_core_news_lg`). No extractor code change required.
    # Empty / "und" language tags resolve to the default model so legacy
    # documents that predate language detection are still NER-ed.
    _LANGUAGE_TO_MODEL: dict[str, str] = {
        "de": "de_core_news_lg",
    }
    _LEGACY_LANGUAGE_TAGS = {"", "und"}

    def __init__(self, language_to_model: dict[str, str] | None = None, default_language: str = "de"):
        self._language_to_model = dict(language_to_model or self._LANGUAGE_TO_MODEL)
        self._default_language = default_language
        self._nlp_by_language: dict[str, "spacy.Language"] = {}
        for lang, model_name in self._language_to_model.items():
            try:
                nlp = spacy.load(model_name, disable=["tagger", "parser", "lemmatizer"])
                self._nlp_by_language[lang] = nlp
                logger.info(
                    "spaCy NER model loaded",
                    language=lang,
                    model=model_name,
                    model_version=nlp.meta.get("version", "unknown"),
                )
            except OSError:
                logger.error(
                    "spaCy model not found — NER disabled for language. "
                    "Install via: python -m spacy download %s",
                    model_name,
                    language=lang,
                )

    # Named entities are at most a short noun phrase. Anything longer is a
    # spaCy false-positive — typically a sentence fragment labeled MISC when
    # the German model encounters non-German text.
    _MAX_ENTITY_CHARS = 80
    _MAX_ENTITY_WORDS = 6

    @property
    def name(self) -> str:
        return "named_entity"

    def _process(self, core):
        """Run spaCy NER pipeline once per document. Returns doc or None."""
        text = core.cleaned_text
        if not text:
            return None
        lang = (core.language or "").lower()
        # Legacy / undetermined documents fall through to the default model
        # so we don't lose NER coverage on adapter outputs that predate
        # language routing.
        if lang in self._LEGACY_LANGUAGE_TAGS:
            lang = self._default_language
        nlp = self._nlp_by_language.get(lang)
        if nlp is None:
            logger.warning(
                "NER skipped: no model loaded for language",
                language=lang,
                source=core.source,
            )
            return None
        return nlp(text)

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
