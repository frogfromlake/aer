"""Probe Language Scope (Phase 122e A17 / F-A17).

Loads `services/analysis-worker/configs/probe_language_scope.yaml` and
exposes a single helper that the processor calls after
`LanguageDetectionExtractor` patches `core.language`.

A scope check returns one of three states:

    * "in_scope"       — source has no entry, OR detected language is allowed.
    * "out_of_scope"   — source has an explicit allow-list and detected
                         language is NOT in it. The processor quarantines.
    * "indeterminate"  — detected language is empty / "und". Treated as
                         in-scope so the document is preserved (the
                         language-detection extractor's own degradation
                         path already records the failure).

The empty-list case (``allowed_languages: []``) is reserved for tests; the
loader rejects it for any source that ships in production.
"""

from pathlib import Path
from typing import Iterable

import yaml

DEFAULT_CONFIG_PATH = (
    Path(__file__).resolve().parent.parent.parent / "configs" / "probe_language_scope.yaml"
)


class ProbeLanguageScope:
    """Per-source language allow-list. Constructed once at worker startup."""

    def __init__(self, source_to_allowed: dict[str, list[str]]):
        self._scope = {
            source: list(langs) for source, langs in source_to_allowed.items()
        }

    @classmethod
    def load(cls, path: Path | None = None) -> "ProbeLanguageScope":
        path = path or DEFAULT_CONFIG_PATH
        with path.open("r", encoding="utf-8") as f:
            data = yaml.safe_load(f) or {}
        sources = data.get("sources") or {}
        if not isinstance(sources, dict):
            raise ValueError(
                f"probe_language_scope.yaml: `sources` must be a mapping, got {type(sources)}"
            )
        for source, langs in sources.items():
            if not isinstance(langs, list) or not all(isinstance(x, str) for x in langs):
                raise ValueError(
                    f"probe_language_scope.yaml: `sources.{source}` must be a list of ISO codes"
                )
        return cls(sources)

    def is_in_scope(self, source: str, detected_language: str) -> bool:
        allowed = self._scope.get(source)
        if allowed is None:
            return True
        if not detected_language or detected_language == "und":
            return True
        return detected_language in allowed

    def allowed_languages(self, source: str) -> list[str] | None:
        allowed = self._scope.get(source)
        return None if allowed is None else list(allowed)

    def sources_with_scope(self) -> Iterable[str]:
        return self._scope.keys()
