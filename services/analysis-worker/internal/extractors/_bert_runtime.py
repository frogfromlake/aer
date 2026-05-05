"""Shared HuggingFace runtime helpers for the Tier-2 sentiment extractors.

Phase 119 ships two BERT-backed extractors (multilingual default per ADR-023
and German-news-domain refinement). Both follow the same recipe:

  1. Apply determinism flags (`torch.manual_seed`, `transformers.set_seed`,
     `torch.use_deterministic_algorithms(True)`) at construction time.
  2. Load tokenizer + sequence-classification model from a pinned revision.
  3. Score 3-class (negative / neutral / positive) softmax outputs to a
     scalar in [-1, 1] by `P(positive) - P(negative)`.

Heavy ML deps (`torch`, `transformers`) are imported under a try/except so
the worker still boots when they are absent — matching the graceful-
degradation pattern the rest of the analysis pipeline uses for spaCy and
`compound-split`. When the deps are missing, every helper here fails closed:
``load_model_and_tokenizer`` returns ``(None, None)``, the extractor sees
that and produces no metric row.
"""

from __future__ import annotations

import hashlib
import structlog

try:
    import torch  # type: ignore[import-not-found]
    import transformers  # type: ignore[import-not-found]
    from transformers import (  # type: ignore[import-not-found]
        AutoModelForSequenceClassification,
        AutoTokenizer,
        set_seed,
    )

    BERT_AVAILABLE = True
except ImportError:  # pragma: no cover - exercised in CI without the package
    torch = None  # type: ignore[assignment]
    transformers = None  # type: ignore[assignment]
    AutoModelForSequenceClassification = None  # type: ignore[assignment]
    AutoTokenizer = None  # type: ignore[assignment]
    set_seed = None  # type: ignore[assignment]
    BERT_AVAILABLE = False

logger = structlog.get_logger()


_DETERMINISM_SEED = 42
_MAX_INPUT_TOKENS = 512


def apply_determinism() -> None:
    """Pin every global PRNG that affects HF inference output.

    Re-applying on every construction is intentional: the determinism CI
    gate runs each extractor twice in the same process and asserts byte-
    identical outputs.
    """
    if not BERT_AVAILABLE:
        return
    torch.manual_seed(_DETERMINISM_SEED)  # type: ignore[union-attr]
    set_seed(_DETERMINISM_SEED)  # type: ignore[misc]
    try:
        torch.use_deterministic_algorithms(True)  # type: ignore[union-attr]
    except RuntimeError:
        # CUDA backends can refuse `use_deterministic_algorithms(True)` if
        # no deterministic kernel exists for an op the model uses. Falling
        # back to the seeds-only path keeps CPU determinism intact, which
        # is what the deployed worker runs on today.
        pass


def load_model_and_tokenizer(
    model_name: str,
    revision: str | None,
):
    """Return ``(tokenizer, model)`` or ``(None, None)`` on any failure.

    `revision` pins the HuggingFace commit SHA. An empty string skips the
    pin (used for tests). All exceptions — from missing transformers, to
    network failures, to missing local cache — collapse to a warning + a
    None pair so the surrounding extractor can degrade gracefully.
    """
    if not BERT_AVAILABLE:
        return None, None
    apply_determinism()
    try:
        kwargs: dict[str, str] = {}
        if revision:
            kwargs["revision"] = revision
        tokenizer = AutoTokenizer.from_pretrained(model_name, **kwargs)  # type: ignore[union-attr]
        model = AutoModelForSequenceClassification.from_pretrained(model_name, **kwargs)  # type: ignore[union-attr]
        model.eval()
        return tokenizer, model
    except Exception as exc:
        logger.warning(
            "HuggingFace model load failed; extractor will produce no metrics",
            model=model_name,
            revision=revision,
            error=str(exc),
            error_type=type(exc).__name__,
        )
        return None, None


def score_three_class_to_scalar(model, tokenizer, text: str) -> float:
    """Run a 3-class classifier and collapse to a scalar in [-1, 1].

    Convention (used by both ``cardiffnlp/twitter-xlm-roberta-base-sentiment``
    and ``mdraw/german-news-sentiment-bert``): the model exposes
    ``id2label`` with three buckets whose names contain ``negative``,
    ``neutral``, and ``positive`` (case-insensitive). The scalar is
    ``P(positive) - P(negative)``; the neutral mass contributes 0.
    """
    with torch.no_grad():  # type: ignore[union-attr]
        inputs = tokenizer(
            text,
            return_tensors="pt",
            truncation=True,
            max_length=_MAX_INPUT_TOKENS,
        )
        logits = model(**inputs).logits
        probs = torch.softmax(logits, dim=-1).squeeze(0)  # type: ignore[union-attr]

    id2label = {idx: str(lbl).lower() for idx, lbl in model.config.id2label.items()}
    pos_prob = 0.0
    neg_prob = 0.0
    for idx, lbl in id2label.items():
        p = float(probs[idx])
        if "pos" in lbl:
            pos_prob += p
        elif "neg" in lbl:
            neg_prob += p
    score = pos_prob - neg_prob
    return max(-1.0, min(1.0, score))


def version_hash(model_name: str, model_revision: str) -> str:
    """Stable identifier mixing model + library versions.

    Used in `SilverEnvelope.extraction_provenance` so that bumping
    `transformers` or `torch` invalidates the provenance hash even when
    the model SHA stays pinned. The hash is intentionally taken on the
    full SHA-256 (not truncated) — these strings live in the Silver
    metadata, not in the per-row Gold tables, so the storage cost is
    negligible and the longer hash makes accidental collisions
    impossible across the lifetime of the corpus.
    """
    transformers_version = transformers.__version__ if BERT_AVAILABLE else "absent"  # type: ignore[union-attr]
    torch_version = torch.__version__ if BERT_AVAILABLE else "absent"  # type: ignore[union-attr]
    seed = f"{model_name}:{model_revision}:{transformers_version}:{torch_version}"
    return hashlib.sha256(seed.encode("utf-8")).hexdigest()
