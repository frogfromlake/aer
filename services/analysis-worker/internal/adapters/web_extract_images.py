"""Image-URL extraction, identity + dedup helpers — extracted from web_extract.py (Phase 141). Heavily unit-tested (JSON-LD image shapes, crop-token identity, dedup)."""

from __future__ import annotations

import logging
import re
from typing import Any
from urllib.parse import urlsplit

from internal.adapters.web_meta import ImageRef

logger = logging.getLogger(__name__)


def _extract_image_url(value: Any) -> str:
    """Return a single URL string from a JSON-LD ``image`` value.

    Phase 122e A25 / F-A25 — JSON-LD ``image`` may be:
      * a bare URL string: ``"https://example.com/foo.jpg"``;
      * a single ``ImageObject`` dict:
        ``{"@type": "ImageObject", "url": "https://..."}`` — the URL
        sits under ``url`` (preferred), ``@id``, or ``contentUrl``;
      * an array of any of the above (the most common pattern when an
        article carries multiple promotional images).

    The previous implementation called ``_stringify(image)`` which
    fell through to ``str(value)`` for a list of ImageObjects,
    producing a stringified Python list-of-dict like
    ``"[{'@type': 'ImageObject', 'url': '...'}]"`` instead of the URL.
    The schema (``WebMeta.image_url: str``) requires a URL string.

    Returns the first URL it finds, or ``""`` if none.
    """
    if value is None:
        return ""
    if isinstance(value, str):
        return value.strip()
    if isinstance(value, dict):
        for key in ("url", "@id", "contentUrl"):
            inner = value.get(key)
            if isinstance(inner, str) and inner.strip():
                return inner.strip()
        return ""
    if isinstance(value, list):
        for item in value:
            url = _extract_image_url(item)
            if url:
                return url
    return ""


# Aspect-ratio / crop path tokens publishers insert to expose ONE image in
# several responsive renditions — e.g. tagesschau's `16x9-1920`, `1x1-840`,
# `4x3`. Stripped when computing an image's identity so the same photo in N
# crops counts once. Matches `WxH` optionally followed by `-<resolution>`.
_IMAGE_CROP_TOKEN_RE = re.compile(r"^\d+x\d+(?:-\d+)?$")


def _image_identity(url: str) -> str:
    """A crop/size-independent identity for an image URL, used to dedupe
    responsive renditions of the same photo (Phase 133 data-quality fix).

    JSON-LD `image` arrays frequently carry one image in multiple aspect-ratio
    crops (schema.org best practice). Counting each rendition makes a constant
    template (tagesschau: always 16x9 + 1x1 + 4x3 of the lead image) read as
    "3 images". This normaliser drops the query string (publishers vary
    `?width=` per rendition) and any aspect-ratio crop path segment, so the
    renditions collapse to one identity while genuinely distinct images
    (different filenames/paths) stay distinct.

    Heuristic, deliberately conservative: a publisher that encodes the size in
    the FILENAME rather than a path segment will not dedupe (an over-count, the
    pre-fix behaviour — never an under-count of real images).
    """
    parts = urlsplit(url.strip())
    segments = [
        s for s in parts.path.split("/") if s and not _IMAGE_CROP_TOKEN_RE.match(s)
    ]
    # Host + crop-stripped path, lowercased. Scheme/query/fragment ignored.
    return f"{parts.netloc.lower()}/{'/'.join(segments).lower()}"


def _dedupe_images(images: list[ImageRef]) -> list[ImageRef]:
    """Collapse responsive renditions of the same image, preserving order and
    keeping the first occurrence. An ImageRef with no URL is always kept (no
    identity to compare on). Never empties a non-empty list, so the field's
    presence/coverage semantics are unchanged."""
    seen: set[str] = set()
    out: list[ImageRef] = []
    for img in images:
        if not img.url:
            out.append(img)
            continue
        key = _image_identity(img.url)
        if key in seen:
            continue
        seen.add(key)
        out.append(img)
    return out
