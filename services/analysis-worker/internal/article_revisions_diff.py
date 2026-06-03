"""Paragraph-level diff extractor for Silent-Edit Diff Substance.
Phase 122d.1 / ADR-032 amendment.

Companion to :mod:`internal.article_revisions` (which manages the
ROW structure of `aer_gold.article_revisions`) and to
:mod:`internal.wayback.snapshot_fetcher` (which fetches the
archived HTML). This module is pure functional: given two HTML
strings (the snapshot bodies at ``revision_index = n-1`` and ``n``),
it computes the paragraph-aligned diff and the headline-change
signal.

Design choices
--------------

* **Paragraph granularity, not sub-paragraph.** Sub-paragraph diffs
  are dominated by re-extraction noise (whitespace shifts,
  boilerplate-trim differences across trafilatura versions). The WP-003
  §5.3 silent-edit signal is about *content* changes, not formatting;
  paragraph-aligned ops carry the analytical signal cleanly.

* **Similarity-based replace detection.** Python ``difflib`` returns
  block-aligned "replace" ops. We project those into per-paragraph
  ``mod`` records when a similar-length pairing exists, otherwise
  we emit them as ``del`` + ``add`` so the dashboard does not
  hallucinate a "modification" where two unrelated paragraphs were
  swapped.

* **Headline extraction priority.** ``<title>`` → ``og:title`` →
  first ``<h1>``. The first non-empty in that order wins. The WP-003
  §5 framing focuses on the title element as the highest-cardinality
  semantic position; ADR-037 (Phase 122d.2) marks that as
  engineering-derived, NOT a methodological canon.

* **Diff payload shape.** ``Array(String)`` in ClickHouse, each
  entry a JSON-encoded ``{op, before?, after?}`` record. We skip
  ``equal`` ops to keep the payload sparse (the dashboard only
  renders the changes).
"""

from __future__ import annotations

import difflib
import json
import re
from dataclasses import dataclass
from typing import Optional

import structlog

logger = structlog.get_logger()

# Trafilatura is the production extractor for Bronze→Silver (see
# `web_extract.py`). The diff path re-runs it on archived HTML so the
# paragraph segmentation stays IDENTICAL to the canonical Silver
# extraction — diffing trafilatura output against trafilatura output
# is the only methodologically-sound comparison (different extractors
# would produce paragraph splits that diff differently for the same
# content). Graceful degradation: if trafilatura is absent at runtime,
# the diff loop is a no-op (mirrors `EXTRACTION_AVAILABLE` in
# `web_extract.py`).
try:
    import trafilatura  # type: ignore

    DIFF_AVAILABLE = True
except ImportError:  # pragma: no cover — production image always ships trafilatura
    trafilatura = None  # type: ignore
    DIFF_AVAILABLE = False


# Similarity threshold for promoting a `replace` opcode to a per-
# paragraph `mod` instead of `del`+`add`. 0.4 catches "rewrote the
# paragraph but kept the topic", 0.0 would catch any pairing
# (including unrelated swaps), 1.0 would require exact match (no
# `mod` would ever fire).
SIMILARITY_THRESHOLD: float = 0.4


# `<h1>` / title extraction — kept regex-based so it works without
# loading the full DOM. trafilatura's `extract_metadata` IS the canon,
# but it requires a network-style fetch context that fails on raw
# strings; the regex chain below is the pragmatic fallback used by
# every webcrawler primitive.
_TITLE_RE = re.compile(r"<title[^>]*>([^<]*)</title>", re.IGNORECASE)
_OG_TITLE_RE = re.compile(
    r"<meta[^>]+property=[\"']og:title[\"'][^>]*content=[\"']([^\"']*)[\"']",
    re.IGNORECASE,
)
_OG_TITLE_RE_ALT = re.compile(
    r"<meta[^>]+content=[\"']([^\"']*)[\"'][^>]*property=[\"']og:title[\"']",
    re.IGNORECASE,
)
_H1_RE = re.compile(r"<h1[^>]*>(.*?)</h1>", re.IGNORECASE | re.DOTALL)
_HTML_TAG_RE = re.compile(r"<[^>]+>")
_WHITESPACE_RE = re.compile(r"\s+")


@dataclass(frozen=True)
class DiffResult:
    """Outcome of one (prev_html, curr_html) diff. Always typed."""

    diff_paragraphs: list[str]
    """JSON-encoded ops, ready for ClickHouse Array(String) insertion."""

    headline_changed: bool
    headline_before: str
    headline_after: str


def _strip_wayback_wrapper(html: str) -> str:
    """Strip the Wayback-Machine-injected banner/script blocks.

    Wayback prepends a toolbar script + CSS block to every archived
    page (delimited by `<!-- BEGIN WAYBACK TOOLBAR INSERT -->` /
    `<!-- END WAYBACK TOOLBAR INSERT -->` markers, or by an
    `id="wm-ipp"` div near the body start). These blocks can contain
    `<title>`-like substrings inside templated script content, which
    a naive regex picks up instead of the article's real title.
    """
    if not html:
        return html
    # Remove toolbar HTML block via the documented markers.
    pattern = re.compile(
        r"<!--\s*BEGIN WAYBACK TOOLBAR INSERT.*?END WAYBACK TOOLBAR INSERT\s*-->",
        re.IGNORECASE | re.DOTALL,
    )
    cleaned = pattern.sub("", html)
    # Remove the JS-injected toolbar container that survives the marker
    # strip on newer Wayback playback paths.
    cleaned = re.sub(
        r'<div[^>]+id=[\"\']wm-ipp[\"\'][^>]*>.*?</div>',
        "",
        cleaned,
        flags=re.IGNORECASE | re.DOTALL,
    )
    return cleaned


def extract_headline(html: str) -> str:
    """Extract the article's title using the canonical priority chain.

    Priority:
      1. ``trafilatura.metadata.extract_metadata().title`` — the
         canonical extractor, robust against the Wayback toolbar that
         confuses naive ``<title>`` regexes (BUG-A).
      2. Regex chain on the Wayback-toolbar-stripped HTML — fallback
         when trafilatura's metadata extractor returns nothing
         (e.g. unusual page structure, missing dependency).

    Returns the empty string when no title can be recovered. The
    caller treats that as "headline unknown" — distinct from an
    empty title element, which is a real publisher signal.
    """
    if not html:
        return ""

    # Step 1 — trafilatura metadata. The canonical extractor knows
    # about JSON-LD / OpenGraph / RDFa / OG:Title in addition to
    # `<title>`, and (crucially) is built to ignore template noise
    # like the Wayback toolbar. Available when DIFF_AVAILABLE (= the
    # production worker image), otherwise we skip straight to the
    # regex fallback so the function still returns something useful
    # in the test image.
    if DIFF_AVAILABLE:
        try:
            from trafilatura.metadata import extract_metadata  # type: ignore

            md = extract_metadata(html)
            if md is not None:
                title = getattr(md, "title", None)
                if isinstance(title, str):
                    cleaned = _normalise_text(title)
                    if cleaned:
                        return cleaned
        except Exception as exc:
            logger.info(
                "diff.trafilatura.metadata_failed",
                error=str(exc),
                error_type=type(exc).__name__,
            )

    # Step 2 — regex fallback on the toolbar-stripped HTML.
    stripped = _strip_wayback_wrapper(html)
    for pattern in (_TITLE_RE, _OG_TITLE_RE, _OG_TITLE_RE_ALT, _H1_RE):
        match = pattern.search(stripped)
        if match:
            candidate = _normalise_text(_HTML_TAG_RE.sub("", match.group(1)))
            if candidate:
                return candidate
    return ""


def extract_paragraphs(html: str) -> list[str]:
    """Return the article body as a list of normalised paragraphs.

    Uses trafilatura at parity with the Bronze→Silver pipeline (same
    library, same flags as ``web_extract._extract_body``). When
    trafilatura is unavailable, returns an empty list — the diff loop
    falls through to a no-op rather than producing meaningless diffs.

    The output is whitespace-normalised so insignificant formatting
    drift (single space vs. double space) does not register as a diff.
    """
    if not DIFF_AVAILABLE or not html:
        return []
    try:
        cleaned = trafilatura.extract(  # type: ignore[union-attr]
            html,
            include_comments=False,
            include_tables=False,
            no_fallback=True,
            with_metadata=False,
            favor_precision=True,
            output_format="txt",
        )
    except Exception as exc:
        logger.info(
            "diff.trafilatura.extract_failed",
            error=str(exc),
            error_type=type(exc).__name__,
        )
        return []
    if not cleaned:
        return []
    paragraphs = [_normalise_text(p) for p in cleaned.split("\n\n")]
    return [p for p in paragraphs if p]


# Sentinel marker for "diff computed but no paragraph-level changes
# detected" — emitted when both inputs parse identically through
# trafilatura. Without this marker the sweep query `WHERE
# length(diff_paragraphs) = 0` would treat the pair as undiffed on
# every subsequent tick and re-fetch the snapshots from IA forever
# (BUG-B). The sentinel makes the row terminal: future sweeps skip
# it. The BFF surface (`GetArticleRevisionDiff`) decodes the
# sentinel into a distinct "snapshots identical" status, distinct
# from the genuine "diff pending" 404 (BUG-10).
SENTINEL_IDENTICAL_OP: dict = {"op": "identical"}


def compute_diff(prev_html: str, curr_html: str) -> DiffResult:
    """Compare two snapshot HTMLs and return the paragraph-aligned diff.

    Empty inputs return a no-op result (`diff_paragraphs=[]`,
    `headline_changed=False`). The caller is expected to skip writing
    when both inputs are empty — there is no meaningful "diff between
    nothing and nothing".

    When both inputs parse but yield identical paragraph content, the
    result carries the `SENTINEL_IDENTICAL_OP` marker so the sweep
    loop's "find undiffed pairs" query stops re-processing the pair
    on every tick (BUG-B). The BFF picks this marker up to surface a
    distinct "snapshots identical" state.
    """
    prev_headline = extract_headline(prev_html)
    curr_headline = extract_headline(curr_html)
    prev_paragraphs = extract_paragraphs(prev_html)
    curr_paragraphs = extract_paragraphs(curr_html)

    ops = _paragraph_diff(prev_paragraphs, curr_paragraphs)
    # A headline change can only be asserted when BOTH sides yield a real
    # title. An empty `prev_headline` means "unknown headline" — extraction
    # recovered nothing, or (for chain-head pairs, revision_index=0) the
    # Silver-now side is the title-less `_silver_text_to_html` wrapper —
    # NOT "the headline was empty and then gained text". Claiming a change
    # from an unknown baseline produced a spurious "− (empty) + <title>" on
    # EVERY chain-head pair, where `prev` is always the title-less wrapper.
    # Requiring both sides non-empty removes that false positive without
    # suppressing any genuine change: every real mid-chain headline edit
    # carries a non-empty `before`.
    headline_changed = (
        bool(prev_headline) and bool(curr_headline) and prev_headline != curr_headline
    )

    # BUG-B sentinel — empty ops AND no headline change = genuinely
    # identical-after-extraction. Mark it so the sweep does not re-
    # process. We still keep `headline_changed=false` in that case.
    if not ops and not headline_changed:
        ops = [SENTINEL_IDENTICAL_OP]

    return DiffResult(
        diff_paragraphs=[json.dumps(op, ensure_ascii=False) for op in ops],
        headline_changed=headline_changed,
        headline_before=prev_headline if headline_changed else "",
        headline_after=curr_headline if headline_changed else "",
    )


def _paragraph_diff(prev: list[str], curr: list[str]) -> list[dict]:
    """Compute paragraph-aligned ops via difflib SequenceMatcher.

    Emits at most one op per logical change. ``equal`` ops are
    skipped to keep the payload sparse. ``replace`` opcodes from
    SequenceMatcher are projected into per-paragraph ``mod`` records
    when a one-to-one similarity match exists (Ratcliff-Obershelp
    similarity ≥ ``SIMILARITY_THRESHOLD``); otherwise into pure
    ``del`` + ``add`` pairs so the dashboard does not invent a
    "modification" where two unrelated paragraphs were swapped.
    """
    matcher = difflib.SequenceMatcher(a=prev, b=curr, autojunk=False)
    out: list[dict] = []
    for tag, i1, i2, j1, j2 in matcher.get_opcodes():
        if tag == "equal":
            continue
        if tag == "delete":
            for k in range(i1, i2):
                out.append({"op": "del", "before": prev[k]})
            continue
        if tag == "insert":
            for k in range(j1, j2):
                out.append({"op": "add", "after": curr[k]})
            continue
        if tag == "replace":
            out.extend(_pair_replace(prev[i1:i2], curr[j1:j2]))
    return out


def _pair_replace(removed: list[str], added: list[str]) -> list[dict]:
    """Match removed↔added paragraphs by similarity; fall back to del+add.

    Greedy pairing: for each removed paragraph, find the best-matching
    added paragraph (similarity above the threshold). Once paired, the
    added paragraph is consumed. Leftovers on either side emit as
    pure ``del`` / ``add``.
    """
    paired_added: set[int] = set()
    ops: list[dict] = []
    for before in removed:
        best_idx: Optional[int] = None
        best_ratio = SIMILARITY_THRESHOLD
        for j, after in enumerate(added):
            if j in paired_added:
                continue
            ratio = difflib.SequenceMatcher(a=before, b=after, autojunk=False).ratio()
            if ratio >= best_ratio:
                best_ratio = ratio
                best_idx = j
        if best_idx is not None:
            ops.append({"op": "mod", "before": before, "after": added[best_idx]})
            paired_added.add(best_idx)
        else:
            ops.append({"op": "del", "before": before})
    for j, after in enumerate(added):
        if j not in paired_added:
            ops.append({"op": "add", "after": after})
    return ops


def _normalise_text(text: str) -> str:
    """Collapse insignificant whitespace; strip leading/trailing space."""
    if not text:
        return ""
    return _WHITESPACE_RE.sub(" ", text).strip()
