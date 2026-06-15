"""Optional NLP-dependency probe for the web-extraction pipeline (Phase 141).

Single source for the optional trafilatura/extruct/htmldate/courlan/readability/
w3lib handles (None when unavailable) + EXTRACTION_AVAILABLE. Set once at import
time; imported by web_extract, web_extract_sources, web_extract_fields."""

from __future__ import annotations

import logging

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Optional-dependency probe.
# ---------------------------------------------------------------------------
try:  # pragma: no cover - import shim only
    import trafilatura  # type: ignore
    import extruct  # type: ignore
    import htmldate  # type: ignore
    import courlan  # type: ignore
    from readability import Document as ReadabilityDocument  # type: ignore
    from w3lib.html import get_base_url  # type: ignore

    EXTRACTION_AVAILABLE = True
except Exception as _extract_import_error:  # pragma: no cover - tested via DLQ path
    trafilatura = None  # type: ignore
    extruct = None  # type: ignore
    htmldate = None  # type: ignore
    courlan = None  # type: ignore
    ReadabilityDocument = None  # type: ignore
    get_base_url = None  # type: ignore
    EXTRACTION_AVAILABLE = False
    _IMPORT_ERROR = _extract_import_error
    logger.warning(
        "web_extract: optional NLP dependencies missing — extraction disabled (%s)",
        _extract_import_error,
    )
