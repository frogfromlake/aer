"""Language Capability Manifest models (ADR-024 / Phase 118a).

The manifest is the system-of-record for per-language analytical capability.
It is loaded once at worker startup from
``services/analysis-worker/configs/language_capabilities.yaml`` and consumed by:

  * ``NamedEntityExtractor``       — model selection per detected language.
  * ``SentimentExtractor``         — Tier-1 lexicon + negation config per language.
  * ``scripts/generate_metric_validity_scaffold.py`` — auto-emits the per-language
    rows of ``infra/clickhouse/seed/metric_validity_scaffold_generated.sql``.
  * The BFF ``?language=`` validator (Go reader of the same YAML).

Validation errors at load time are fatal — the worker refuses to start with an
invalid manifest, never silently falling back to a hard-coded default.

See ADR-024 for the architectural decision and the manifest-version evolution
policy.
"""

from __future__ import annotations

from pathlib import Path
from typing import Literal

import yaml
from pydantic import BaseModel, ConfigDict, Field, ValidationError, field_validator, model_validator


class ConfigurationError(RuntimeError):
    """Raised when the Language Capability Manifest fails to load or validate.

    Distinct from ``pydantic.ValidationError`` so callers can handle manifest
    issues separately from extractor-payload validation issues.
    """


class NerCapability(BaseModel):
    """Per-language NER configuration.

    ``tier`` follows ADR-016: 1.5 is the alias-resolved span-emitting NER
    that ships in Phase 118; deeper Tier-2 linkers will register as 2.0+.
    """

    model_config = ConfigDict(extra="forbid")

    tier: float
    model: str
    model_version: str
    provenance: str = ""


class NegationConfig(BaseModel):
    """Language-specific negation cues consumed by ``SentimentExtractor``.

    Replaces the hard-coded ``_NEGATION_CUES`` / ``_CLAUSE_BOUNDARY_DEPS`` /
    ``_NEGATION_DEPS`` constants from Phase 117 by lifting them into the
    manifest. Every entry is required so that adding a language forces an
    explicit negation-handling decision.
    """

    model_config = ConfigDict(extra="forbid")

    particles: list[str] = Field(default_factory=list)
    clause_boundaries: list[str] = Field(default_factory=list)
    spacy_neg_dep: str = ""
    spacy_neg_deps_extra: list[str] = Field(default_factory=list)
    clause_boundary_deps: list[str] = Field(default_factory=list)


# Sentiment tier-1 method values. Only ``lexicon`` is implemented today
# (SentiWS for German). Future entries (``rule_based``, ``hybrid_lexicon``)
# extend this Literal at the same time as the implementing extractor lands.
SentimentTier1Method = Literal["lexicon"]


class SentimentTier1Capability(BaseModel):
    """Per-language Tier-1 sentiment configuration."""

    model_config = ConfigDict(extra="forbid")

    tier: float = 1.0
    method: SentimentTier1Method
    lexicon: str
    features: list[str] = Field(default_factory=list)
    negation: NegationConfig
    metric_name: str

    @field_validator("features")
    @classmethod
    def _validate_features(cls, v: list[str]) -> list[str]:
        # The current extractor recognises three feature flags. Unknown flags
        # are caught here so a typo cannot silently disable a sub-feature.
        known = {"negation_dependency", "compound_split", "custom_lexicon"}
        unknown = [f for f in v if f not in known]
        if unknown:
            raise ValueError(
                f"unknown sentiment_tier1.features values: {unknown}; "
                f"expected subset of {sorted(known)}"
            )
        return v


class SentimentTier2DefaultCapability(BaseModel):
    """Per-language Tier-2 default sentiment placeholder (Phase 119 consumer)."""

    model_config = ConfigDict(extra="forbid")

    tier: float = 2.0
    method: str
    provided_by: str
    metric_name: str


class SentimentTier2RefinementCapability(BaseModel):
    """Per-language Tier-2.5 refinement placeholder (Phase 119 consumer)."""

    model_config = ConfigDict(extra="forbid")

    tier: float = 2.5
    method: str
    model: str
    model_revision: str = ""
    metric_name: str


class CulturalCalendarRef(BaseModel):
    model_config = ConfigDict(extra="forbid")

    region_default: str
    file: str


class LanguageCapability(BaseModel):
    """Per-language capability block."""

    model_config = ConfigDict(extra="forbid")

    iso_code: str
    display_name: str
    ner: NerCapability | None = None
    sentiment_tier1: SentimentTier1Capability | None = None
    sentiment_tier2_default: SentimentTier2DefaultCapability | None = None
    sentiment_tier2_refinement: SentimentTier2RefinementCapability | None = None
    cultural_calendar: CulturalCalendarRef | None = None
    notes: list[str] = Field(default_factory=list)


class MultilingualBertCapability(BaseModel):
    """Shared multilingual BERT model registration (Phase 119 consumer)."""

    model_config = ConfigDict(extra="forbid")

    model: str
    model_revision: str = ""
    supported_languages: list[str] = Field(default_factory=list)


class SharedCapability(BaseModel):
    """Cross-language shared resources."""

    model_config = ConfigDict(extra="forbid")

    multilingual_bert: MultilingualBertCapability | None = None


class CapabilityManifest(BaseModel):
    """Root manifest model. Loaded once at worker startup."""

    model_config = ConfigDict(extra="forbid")

    manifest_version: int
    languages: dict[str, LanguageCapability]
    shared: SharedCapability = Field(default_factory=SharedCapability)

    @field_validator("manifest_version")
    @classmethod
    def _supported_version(cls, v: int) -> int:
        # ADR-024 versioning commitment: only manifest_version=1 is recognised
        # today. Future bumps will arrive with explicit migration guidance.
        if v != 1:
            raise ValueError(
                f"unsupported manifest_version {v}; this worker recognises only version 1"
            )
        return v

    @model_validator(mode="after")
    def _no_metric_name_collisions(self) -> "CapabilityManifest":
        """No two languages may declare the same metric_name for different methods.

        ClickHouse aggregates by ``metric_name`` only — collisions silently
        merge rows from semantically incompatible extractors. This guard keeps
        the manifest honest at load time.
        """
        seen: dict[str, tuple[str, str]] = {}
        for lang_code, lang in self.languages.items():
            buckets: list[tuple[str, str]] = []
            if lang.sentiment_tier1 is not None:
                buckets.append((lang.sentiment_tier1.metric_name, lang.sentiment_tier1.method))
            if lang.sentiment_tier2_default is not None:
                buckets.append((lang.sentiment_tier2_default.metric_name, lang.sentiment_tier2_default.method))
            if lang.sentiment_tier2_refinement is not None:
                buckets.append((lang.sentiment_tier2_refinement.metric_name, lang.sentiment_tier2_refinement.method))
            for metric_name, method in buckets:
                prior = seen.get(metric_name)
                if prior is not None and prior != (lang_code, method):
                    prior_lang, prior_method = prior
                    if prior_method != method:
                        raise ValueError(
                            f"metric_name collision: {metric_name!r} declared with "
                            f"method={prior_method!r} ({prior_lang}) and method={method!r} "
                            f"({lang_code}); same metric must use the same method"
                        )
                seen[metric_name] = (lang_code, method)
        return self


_DEFAULT_MANIFEST_PATH = (
    Path(__file__).resolve().parent.parent.parent / "configs" / "language_capabilities.yaml"
)


def load_manifest(path: Path | None = None) -> CapabilityManifest:
    """Load and validate the Language Capability Manifest.

    Raises ``ConfigurationError`` with the underlying parse / validation cause
    so the worker startup path can refuse to boot with a structured error.
    """
    manifest_path = path or _DEFAULT_MANIFEST_PATH
    if not manifest_path.exists():
        raise ConfigurationError(
            f"language capability manifest not found at {manifest_path}"
        )
    try:
        with open(manifest_path, encoding="utf-8") as f:
            raw = yaml.safe_load(f)
    except yaml.YAMLError as exc:
        raise ConfigurationError(
            f"language capability manifest at {manifest_path} is not valid YAML: {exc}"
        ) from exc
    if not isinstance(raw, dict):
        raise ConfigurationError(
            f"language capability manifest at {manifest_path} must be a mapping"
        )
    try:
        return CapabilityManifest.model_validate(raw)
    except ValidationError as exc:
        raise ConfigurationError(
            f"language capability manifest at {manifest_path} failed schema validation: {exc}"
        ) from exc


__all__ = [
    "CapabilityManifest",
    "ConfigurationError",
    "CulturalCalendarRef",
    "LanguageCapability",
    "MultilingualBertCapability",
    "NegationConfig",
    "NerCapability",
    "SentimentTier1Capability",
    "SentimentTier2DefaultCapability",
    "SentimentTier2RefinementCapability",
    "SharedCapability",
    "load_manifest",
]
