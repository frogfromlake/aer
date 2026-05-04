import structlog
import spacy

from internal.extractors.base import GoldEntity, GoldEntityLink, GoldMetric, ExtractionResult
from internal.extractors.entity_linking import WikidataAliasIndex
from internal.models.language_capability import CapabilityManifest, load_manifest

logger = structlog.get_logger()


class NamedEntityExtractor:
    """
    Named Entity Recognition using spaCy de_core_news_lg.

    **Provisional (Phase 42)** — This is a proof-of-concept, not a
    scientifically validated implementation.

    Implements the unified MetricExtractor protocol (Phase 52): a single
    extract_all() call returns the entity_count metric, GoldEntity records,
    and (Phase 118) GoldEntityLink records in one pass. Stateless between
    documents — no mutable caching.

    Produces:
    - metric_name = "entity_count": Total number of entities found in the text.
    - GoldEntity records: Raw entity spans stored in aer_gold.entities.
    - GoldEntityLink records: Wikidata QIDs for spans that resolve via the
      alias index (Phase 118). Only successfully linked spans are emitted —
      `aer_gold.entities` remains the canonical span SoT and `entity_links`
      is its LEFT-JOIN sidecar. Confidence weights (1.0 / 0.85 / 0.7 for
      label / altLabel / accent-fold matches) are heuristic Tier-1.5
      defaults pending the WP-002 §4.2 annotation study.

    Limitations (to be addressed with interdisciplinary collaboration, §13.5):
    - spaCy NER on RSS feed descriptions (short, truncated summaries) will
      produce different results than NER on full articles. Recall is expected
      to be significantly lower on short texts.
    - The model version, entity taxonomy (PER, ORG, LOC, MISC), and
      post-processing pipeline will evolve with the research.
    - German compound words and named entities spanning multiple tokens
      are handled by spaCy's statistical model, not by rule-based patterns.
    - The de_core_news_lg model (~500MB) is trained on German news text,
      which is a reasonable domain match for Probe 0 but may not generalize
      to other domains or registers.
    """

    # Phase 116: language-routing. As of Phase 118a (ADR-024) the routing
    # table comes from the Language Capability Manifest — see
    # `configs/language_capabilities.yaml`. Adding a new language is a
    # manifest YAML edit plus the spaCy model in requirements.txt; this
    # extractor stays untouched.
    # Empty / "und" language tags resolve to the default model so legacy
    # documents that predate language detection are still NER-ed.
    _LEGACY_LANGUAGE_TAGS = {"", "und"}

    def __init__(
        self,
        manifest: CapabilityManifest | None = None,
        default_language: str = "de",
        alias_index: WikidataAliasIndex | None = None,
    ):
        self._manifest = manifest if manifest is not None else load_manifest()
        self._default_language = default_language
        self._alias_index = alias_index
        # Build the routing map from manifest entries that declare an `ner`
        # block. Languages without `ner` are simply absent from the map and
        # fall through to the structured-warning skip path at request time.
        language_to_model: dict[str, str] = {
            lang_code: lang.ner.model
            for lang_code, lang in self._manifest.languages.items()
            if lang.ner is not None
        }
        self._language_to_model = language_to_model
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
                "NER skipped: no manifest entry for language",
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

        entity_links: list[GoldEntityLink] = []
        if self._alias_index is not None and valid_ents:
            lookup_language = (core.language or "").lower() or self._default_language
            for ent in valid_ents:
                candidate = self._alias_index.lookup(ent.text, lookup_language)
                if candidate is None:
                    logger.debug(
                        "entity_link_skipped",
                        entity_text=ent.text,
                        language=lookup_language,
                        article_id=article_id,
                        reason="no_candidate_above_threshold",
                    )
                    continue
                entity_links.append(
                    GoldEntityLink(
                        timestamp=core.timestamp,
                        article_id=article_id,
                        entity_text=ent.text,
                        entity_label=ent.label_,
                        wikidata_qid=candidate.wikidata_qid,
                        link_confidence=candidate.confidence,
                        link_method=candidate.method,
                    )
                )

        return ExtractionResult(metrics=metrics, entities=entities, entity_links=entity_links)
