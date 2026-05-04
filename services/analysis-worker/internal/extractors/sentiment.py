import hashlib
import structlog
from pathlib import Path

import yaml

try:  # spaCy is loaded by NER too — failure is non-fatal (graceful degradation).
    import spacy
except ImportError:  # pragma: no cover - spaCy is a hard runtime dep
    spacy = None  # type: ignore[assignment]

try:
    from compound_split import char_split  # type: ignore[import-not-found]
    _COMPOUND_SPLIT_AVAILABLE = True
except ImportError:  # pragma: no cover - exercised in CI without the package
    char_split = None  # type: ignore[assignment]
    _COMPOUND_SPLIT_AVAILABLE = False

from internal.extractors.base import GoldMetric, ExtractionResult
from internal.models.language_capability import (
    CapabilityManifest,
    SentimentTier1Capability,
    load_manifest,
)

logger = structlog.get_logger()

# Default path to SentiWS data files. Overridable for testing.
_DEFAULT_SENTIWS_DIR = Path(__file__).resolve().parent.parent.parent / "data" / "sentiws"
_DEFAULT_CUSTOM_LEXICON = Path(__file__).resolve().parent.parent.parent / "data" / "custom_lexicon.yaml"

# Phase 118a (ADR-024): the negation cues, neg-dep labels, and clause-boundary
# dep labels that used to live as module-level frozensets are now lifted into
# `configs/language_capabilities.yaml` under each language's
# `sentiment_tier1.negation` block. The constants below are derived from the
# manifest at construction time — see `SentimentExtractor.__init__`.


def _load_sentiws(directory: Path) -> dict[str, float]:
    """Load SentiWS v2.0 lexicon files into a word→polarity dict."""
    lexicon: dict[str, float] = {}

    for filename in ("SentiWS_v2.0_Positive.txt", "SentiWS_v2.0_Negative.txt"):
        filepath = directory / filename
        if not filepath.exists():
            logger.warning("SentiWS file not found — sentiment extraction disabled", path=str(filepath))
            return {}

        with open(filepath, encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue

                parts = line.split("\t")
                if len(parts) < 2:
                    continue

                word_pos = parts[0]
                word = word_pos.split("|")[0].strip().lower()
                try:
                    weight = float(parts[1])
                except ValueError:
                    continue

                lexicon[word] = weight

                if len(parts) >= 3 and parts[2].strip():
                    for inflection in parts[2].split(","):
                        inflection = inflection.strip().lower()
                        if inflection:
                            lexicon[inflection] = weight

    return lexicon


def _load_custom_lexicon(path: Path) -> dict[str, float]:
    """
    Load the operator-managed custom lexicon (Phase 117).

    The file is a YAML mapping of `word: polarity` pairs (polarity in [-1, 1]).
    An empty / missing file yields an empty dict — the file is *checked in*
    empty by default; entries are added out-of-band per the operations
    playbook.
    """
    if not path.exists():
        return {}
    try:
        with open(path, encoding="utf-8") as f:
            raw = yaml.safe_load(f) or {}
    except yaml.YAMLError as exc:
        logger.warning("custom_lexicon.yaml parse failed — ignored", path=str(path), error=str(exc))
        return {}
    if not isinstance(raw, dict):
        logger.warning("custom_lexicon.yaml is not a mapping — ignored", path=str(path))
        return {}
    out: dict[str, float] = {}
    for k, v in raw.items():
        try:
            out[str(k).strip().lower()] = float(v)
        except (TypeError, ValueError):
            logger.warning("custom_lexicon entry skipped — not numeric", word=k, value=v)
    return out


def _compute_lexicon_hash(lexicon: dict[str, float]) -> str:
    items = sorted(lexicon.items())
    content = "|".join(f"{w}:{v}" for w, v in items)
    return hashlib.sha256(content.encode("utf-8")).hexdigest()[:16]


def _load_spacy_de() -> "spacy.Language | None":
    """Load `de_core_news_lg` with parser+tagger for dependency analysis.

    Returns None on failure — sentiment then falls back to bag-of-words
    scoring without negation handling (graceful degradation).
    """
    if spacy is None:
        return None
    try:
        # Disable NER and lemmatizer; we only need tagger + parser for the
        # dependency-tree walk that drives negation scope detection.
        return spacy.load("de_core_news_lg", disable=["ner", "lemmatizer"])
    except OSError:
        logger.warning(
            "spaCy de_core_news_lg not available — falling back to "
            "bag-of-words sentiment without negation handling"
        )
        return None


class SentimentExtractor:
    """
    Lexicon-based German sentiment scoring (SentiWS v2.0, CC-BY-SA).

    Phase 117 hardenings (Tier 1 — deterministic, no model inference):
      * **Dependency-based negation scope.** For each polarity-scored token,
        walk the spaCy dependency tree to detect whether it sits within the
        scope of a negation cue (the German particles `nicht`, `kein/-e/...`,
        `niemals`, `nie`, `nirgends`, `kaum` and any token tagged with the
        `neg` dep relation). Polarity is inverted within the cue's
        syntactic subtree; clause-coordinating conjunctions clamp the scope
        so embedded clauses don't bleed.
      * **German compound decomposition.** Tokens unmatched by SentiWS are
        attempted with `compound-split` (frequency-list-based, deterministic).
        On a clean split into two SentiWS-known sub-words, the polarity is
        the mean of the sub-word polarities.
      * **Custom lexicon hook.** `data/custom_lexicon.yaml` is merged on top
        of SentiWS at startup — the designated out-of-band path for adding
        neologisms (`toxisch`, `Querdenker`, `Wutbürger`) without patching
        the versioned SentiWS file.

    The metric name `sentiment_score_sentiws` makes ADR-016's dual-metric
    pattern (Tier 1 alongside Tier 2 BERT in Phase 119) lexically explicit.
    """

    # Default metric name; overridden by the manifest entry at construction.
    METRIC_NAME = "sentiment_score_sentiws"

    def __init__(
        self,
        sentiws_dir: Path | None = None,
        custom_lexicon_path: Path | None = None,
        nlp: "spacy.Language | None" = None,
        manifest: CapabilityManifest | None = None,
    ):
        # Phase 118a: read language routing + negation config from the
        # Capability Manifest (ADR-024). The current Tier-1 implementation
        # is SentiWS + spaCy German, so we resolve to the `de` block; other
        # languages will register their own Tier-1 lexicons in subsequent
        # phases without touching this extractor.
        self._manifest = manifest if manifest is not None else load_manifest()
        de_block = self._manifest.languages.get("de")
        de_sentiment: SentimentTier1Capability | None = (
            de_block.sentiment_tier1 if de_block is not None else None
        )

        if de_sentiment is not None:
            self.METRIC_NAME = de_sentiment.metric_name
            self._features = set(de_sentiment.features)
            self._negation_cues: frozenset[str] = frozenset(
                p.lower() for p in de_sentiment.negation.particles
            )
            self._negation_deps: frozenset[str] = frozenset(
                d for d in (
                    [de_sentiment.negation.spacy_neg_dep] + de_sentiment.negation.spacy_neg_deps_extra
                )
                if d
            )
            self._clause_boundary_deps: frozenset[str] = frozenset(
                de_sentiment.negation.clause_boundary_deps
            )
        else:
            # Manifest does not declare German Tier-1 sentiment — extractor
            # produces no output rather than silently using stale defaults.
            self._features = set()
            self._negation_cues = frozenset()
            self._negation_deps = frozenset()
            self._clause_boundary_deps = frozenset()

        # Phase 116: language guard derived from manifest membership. Legacy
        # `und`/`""` tags fall through so pre-detection documents keep working.
        supported = {
            code for code, lang in self._manifest.languages.items()
            if lang.sentiment_tier1 is not None and lang.sentiment_tier1.method == "lexicon"
        }
        self._supported_languages = supported | {"und", ""}

        directory = sentiws_dir or _DEFAULT_SENTIWS_DIR
        self._sentiws = _load_sentiws(directory)
        custom_path = custom_lexicon_path or _DEFAULT_CUSTOM_LEXICON
        # `custom_lexicon` is gated by the manifest feature flag so a language
        # block can disable the override surface without removing the file.
        self._custom = (
            _load_custom_lexicon(custom_path)
            if "custom_lexicon" in self._features
            else {}
        )

        # Custom entries override SentiWS (intentional — this is the operator
        # escape hatch for cases where SentiWS scores a neologism wrong).
        self._lexicon: dict[str, float] = {**self._sentiws, **self._custom}
        self._lexicon_hash = (
            _compute_lexicon_hash(self._lexicon) if self._lexicon else "empty"
        )
        # spaCy parser is optional — sentiment still produces metrics without
        # it, but loses negation-scope detection.
        self._compound_split_active = (
            "compound_split" in self._features and _COMPOUND_SPLIT_AVAILABLE
        )
        self._negation_dependency_active = "negation_dependency" in self._features
        self._nlp = (
            nlp
            if nlp is not None
            else (_load_spacy_de() if self._negation_dependency_active else None)
        )

        if self._lexicon:
            logger.info(
                "SentiWS lexicon loaded",
                sentiws_entries=len(self._sentiws),
                custom_entries=len(self._custom),
                lexicon_hash=self._lexicon_hash,
                parser_loaded=self._nlp is not None,
                compound_split=self._compound_split_active,
                negation_dependency=self._negation_dependency_active,
                manifest_features=sorted(self._features),
            )
        else:
            logger.warning("SentiWS lexicon empty — sentiment extractor will produce no metrics")

    @property
    def name(self) -> str:
        return "sentiment"

    @property
    def lexicon_hash(self) -> str:
        return self._lexicon_hash

    @property
    def version_hash(self) -> str:
        # version_hash mixes lexicon contents with the structural-improvement
        # markers (parser + compound split) so Phase 117's algorithmic shift
        # is reflected in extraction provenance independent of SentiWS data.
        marker = f"v117:parser={int(self._nlp is not None)}:cs={int(self._compound_split_active)}"
        return hashlib.sha256(
            f"{self._lexicon_hash}|{marker}".encode("utf-8")
        ).hexdigest()[:16]

    # ---------------------------------------------------------------- helpers

    def _split_compound(self, token: str) -> float | None:
        """Frequency-based German compound split → averaged polarity, or None.

        Returns None when:
          * the manifest disables `compound_split`, or
          * `compound-split` is unavailable, or
          * the token does not split cleanly, or
          * any sub-word is missing from the lexicon.

        Compound splitter output is deterministic given the bundled frequency
        list — Tier-1 reproducibility is preserved.
        """
        if not self._compound_split_active or char_split is None:
            return None
        try:
            candidates = char_split.split_compound(token)
        except Exception:  # pragma: no cover - defensive; library is permissive
            return None
        if not candidates:
            return None
        # `compound-split` returns [(score, head, tail), ...] sorted by score.
        for cand in candidates:
            if not isinstance(cand, (list, tuple)) or len(cand) < 3:
                continue
            score, head, tail = cand[0], cand[1], cand[2]
            if score is None or score < 0:
                continue
            head_l = str(head).lower()
            tail_l = str(tail).lower()
            if head_l in self._lexicon and tail_l in self._lexicon:
                return (self._lexicon[head_l] + self._lexicon[tail_l]) / 2.0
        return None

    @staticmethod
    def _strip_punct(token: str) -> str:
        return token.strip(".,;:!?\"'()[]{}«»–—…")

    def _is_negation_token(self, token) -> bool:
        return token.dep_ in self._negation_deps or token.lower_ in self._negation_cues

    def _negation_scope(self, doc) -> set[int]:
        """Token indices whose polarity should be inverted.

        For each negation cue, the inverted set is the cue's syntactic head's
        subtree (covering pre-verbal adjectives, post-verbal complements, and
        the verb/predicate itself), clamped at clause-coordinating boundaries
        so an embedded `dass`/`weil`/`obwohl` clause is not punished by a
        matrix-clause `nicht`. A small fallback catches the common case where
        the parser attaches the cue directly to the predicate.
        """
        inverted: set[int] = set()
        for tok in doc:
            if not self._is_negation_token(tok):
                continue
            anchor = tok.head if tok.head is not None and tok.head is not tok else tok
            for descendant in anchor.subtree:
                # Don't cross a clause boundary going *into* an embedded clause:
                # the descendant's path back to the anchor passes through a
                # subordinating marker → not in scope.
                if descendant.i == anchor.i:
                    inverted.add(descendant.i)
                    continue
                if self._crosses_clause_boundary(descendant, anchor):
                    continue
                inverted.add(descendant.i)
        return inverted

    def _crosses_clause_boundary(self, node, anchor) -> bool:
        """True iff the path from `node` up to `anchor` crosses a clause head.

        A clause head is a token that has a clause-boundary dep child in the
        parse — the German subordinators `dass`, `weil`, `obwohl`, `wenn`, ...
        are attached as `mark` (UD) or `cp` (TIGER), coordinating
        conjunctions as `cc` / `cd`. Anything *below* such a head sits in a
        separate clause from anything *above* it. The exact dep-label set is
        manifest-driven (`sentiment_tier1.negation.clause_boundary_deps`).
        """
        cur = node
        boundary_deps = self._clause_boundary_deps
        while cur is not None and cur.i != anchor.i:
            # Direct dep-relation check first (covers cases where the node
            # itself is the boundary marker, e.g. coordinated VPs).
            if cur.dep_ in boundary_deps:
                return True
            for child in cur.children:
                if child.dep_ in boundary_deps:
                    return True
            parent = cur.head
            if parent is None or parent is cur:
                return False
            cur = parent
        return False

    def _score_with_parser(self, text: str) -> list[float]:
        """Per-token polarities, inverted within negation scope."""
        doc = self._nlp(text)  # type: ignore[misc]
        inverted = self._negation_scope(doc)
        scores: list[float] = []
        for tok in doc:
            cleaned = self._strip_punct(tok.lower_)
            if not cleaned:
                continue
            polarity = self._lexicon.get(cleaned)
            if polarity is None:
                polarity = self._split_compound(cleaned)
            if polarity is None:
                continue
            if tok.i in inverted:
                polarity = -polarity
            scores.append(polarity)
        return scores

    def _score_bag_of_words(self, text: str) -> list[float]:
        """Fallback path when spaCy parser is unavailable."""
        scores: list[float] = []
        for token in text.lower().split():
            cleaned = self._strip_punct(token)
            if not cleaned:
                continue
            polarity = self._lexicon.get(cleaned)
            if polarity is None:
                polarity = self._split_compound(cleaned)
            if polarity is None:
                continue
            scores.append(polarity)
        return scores

    # ---------------------------------------------------------------- entry

    def extract_all(self, core, article_id: str | None) -> ExtractionResult:
        if not self._lexicon:
            return ExtractionResult()

        text = core.cleaned_text
        if not text:
            return ExtractionResult()

        lang = (core.language or "").lower()
        if lang not in self._supported_languages:
            return ExtractionResult()

        if self._nlp is not None:
            scores = self._score_with_parser(text)
        else:
            scores = self._score_bag_of_words(text)

        if not scores:
            sentiment = 0.0
        else:
            sentiment = sum(scores) / len(scores)
        sentiment = max(-1.0, min(1.0, sentiment))

        return ExtractionResult(
            metrics=[
                GoldMetric(
                    timestamp=core.timestamp,
                    value=round(sentiment, 4),
                    source=core.source,
                    metric_name=self.METRIC_NAME,
                    article_id=article_id,
                ),
            ]
        )
