import hashlib
import structlog
from pathlib import Path

from internal.extractors.base import GoldMetric, ExtractionResult

logger = structlog.get_logger()

# Default path to SentiWS data files. Overridable for testing.
_DEFAULT_SENTIWS_DIR = Path(__file__).resolve().parent.parent.parent / "data" / "sentiws"


def _load_sentiws(directory: Path) -> dict[str, float]:
    """
    Load SentiWS v2.0 lexicon files into a word→polarity dict.

    SentiWS format: ``word|POS\\tweight\\tinflection1,inflection2,...``
    Both the base form and all inflections receive the same polarity weight.

    Returns an empty dict if the files are not found (graceful degradation).
    """
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

                # Parse "word|POS" → extract word
                word_pos = parts[0]
                word = word_pos.split("|")[0].strip().lower()
                try:
                    weight = float(parts[1])
                except ValueError:
                    continue

                lexicon[word] = weight

                # Parse inflections if present
                if len(parts) >= 3 and parts[2].strip():
                    for inflection in parts[2].split(","):
                        inflection = inflection.strip().lower()
                        if inflection:
                            lexicon[inflection] = weight

    return lexicon


def _compute_lexicon_hash(lexicon: dict[str, float]) -> str:
    """Deterministic SHA-256 hash of the lexicon contents for auditability."""
    items = sorted(lexicon.items())
    content = "|".join(f"{w}:{v}" for w, v in items)
    return hashlib.sha256(content.encode("utf-8")).hexdigest()[:16]


class SentimentExtractor:
    """
    Lexicon-based sentiment scoring using SentiWS (Leipzig University, CC-BY-SA).

    **Provisional (Phase 42)** — This is a proof-of-concept, not a
    scientifically validated implementation.

    Scoring algorithm:
    - Tokenize cleaned_text by whitespace.
    - Lowercase each token and strip punctuation.
    - Look up each token in the SentiWS lexicon.
    - Score = mean of matched token polarities (0.0 if no matches).
    - Clamp to [-1.0, 1.0].

    Produces:
    - metric_name = "sentiment_score": Mean word-level polarity.

    Lexicon version provenance is exposed via the ``lexicon_hash`` property and
    recorded in the Silver envelope's ``extraction_provenance`` field — not as a
    ClickHouse metric.

    Limitations (to be addressed with interdisciplinary collaboration, §13.5):
    - SentiWS is a word-level polarity lexicon. It assigns fixed sentiment
      weights to individual words without considering context.
    - Does NOT handle: negation ("nicht gut" scores positive), irony,
      sarcasm, domain-specific jargon, compositionality, or multi-word
      expressions.
    - Whitespace tokenization is naive — does not handle German compound
      words (e.g., "Klimaschutzpaket" is one token, not three).
    - The scoring algorithm (mean of matched polarities) is a placeholder.
      Normalized, weighted, or positional scoring may be more appropriate.
    - The specific lexicon, scoring algorithm, and normalization WILL change
      when CSS researchers (§13.5) provide validated alternatives.
    """

    def __init__(self, sentiws_dir: Path | None = None):
        directory = sentiws_dir or _DEFAULT_SENTIWS_DIR
        self._lexicon = _load_sentiws(directory)
        self._lexicon_hash = _compute_lexicon_hash(self._lexicon) if self._lexicon else "empty"

        if self._lexicon:
            logger.info(
                "SentiWS lexicon loaded",
                entries=len(self._lexicon),
                lexicon_hash=self._lexicon_hash,
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
        return self._lexicon_hash

    def extract_all(self, core, article_id: str | None) -> ExtractionResult:
        if not self._lexicon:
            return ExtractionResult()

        text = core.cleaned_text
        if not text:
            return ExtractionResult()

        # Naive whitespace tokenization + lowercase + punctuation strip
        tokens = text.lower().split()
        scores: list[float] = []

        for token in tokens:
            # Strip common punctuation from token boundaries
            cleaned = token.strip(".,;:!?\"'()[]{}«»–—…")
            if cleaned in self._lexicon:
                scores.append(self._lexicon[cleaned])

        if not scores:
            sentiment = 0.0
        else:
            sentiment = sum(scores) / len(scores)

        # Clamp to [-1, 1]
        sentiment = max(-1.0, min(1.0, sentiment))

        return ExtractionResult(
            metrics=[
                GoldMetric(
                    timestamp=core.timestamp,
                    value=round(sentiment, 4),
                    source=core.source,
                    metric_name="sentiment_score",
                    article_id=article_id,
                ),
            ]
        )
