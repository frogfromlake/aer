"""Article-URL-pattern inference + CMS detection — extracted from audit_source.py (Phase 141). Pattern inference and CMS detection are mutually coupled (CMS templates drive inference; date-walking uses non-article-link extraction), so they live in one module."""

import logging
import re
from typing import Any, Optional
from urllib.parse import urljoin, urlparse


from audit_probe import _A_HREF_RE, _ASSET_EXTENSIONS, _GENERATOR_META_RE

logger = logging.getLogger(__name__)


def _extract_article_url_candidates(
    body: str,
    base_url: str,
    *,
    self_url: Optional[str] = None,
) -> list[str]:
    """Parse ``<a href>`` links from a publisher page and return the
    subset that plausibly point to articles.

    Filters applied:
      * Same host as ``base_url`` (cross-domain links are navigation
        / partner widgets, not articles).
      * Not the page's own URL (``self_url``) — the HTML sitemap often
        includes its own permalink in the navigation.
      * Path depth ≥ 2 (root + at least one section, articles are
        almost never at root).
      * Path does NOT end in an asset extension.
      * Path is not a query-only or fragment-only link.

    Used by :func:`_infer_article_url_pattern` to derive a publisher-
    specific regex when the operator accepts an html_sitemap or
    archive_index diff.
    """
    if not body:
        return []
    parsed_base = urlparse(base_url)
    if not parsed_base.netloc:
        return []
    base_host = parsed_base.netloc.lower().removeprefix("www.")
    seen: set[str] = set()
    out: list[str] = []
    for match in _A_HREF_RE.finditer(body):
        href = match.group(1).strip()
        if not href or href.startswith(("mailto:", "tel:", "javascript:", "#")):
            continue
        absolute = urljoin(base_url, href)
        parsed = urlparse(absolute)
        if parsed.scheme not in ("http", "https"):
            continue
        host = parsed.netloc.lower().removeprefix("www.")
        if host != base_host:
            continue
        if self_url and absolute.rstrip("/") == self_url.rstrip("/"):
            continue
        path = parsed.path or ""
        if not path or path == "/":
            continue
        if path.lower().endswith(_ASSET_EXTENSIONS):
            continue
        # Path depth ≥ 2 (e.g. /section/slug, not just /about).
        if path.count("/") < 2:
            continue
        # Dedupe by canonical form (drop trailing slash, lowercase host).
        canon = absolute.rstrip("/").lower()
        if canon in seen:
            continue
        seen.add(canon)
        out.append(absolute)
    return out


def _infer_article_url_pattern(
    article_urls: list[str],
    homepage_origin: str,
    *,
    min_sample: int = 5,
) -> Optional[str]:
    """Derive a conservative regex that matches the supplied article
    URLs from a single host. Returns ``None`` when no high-confidence
    pattern can be inferred (the operator will see the ``EDIT-ME``
    placeholder and write one manually).

    Three patterns, tried in order of specificity:

    1. **Slug-with-numeric-id + ``.html``** (e.g. tagesschau:
       ``/inland/.../foo-bar-NNN.html``). Conservative AND specific.
    2. **Common path prefix + ``.html`` extension** when ≥ 80 % of
       URLs share the same first path segment (e.g. bundesregierung:
       ``/breg-de/aktuelles/...``).
    3. **Common path prefix only** when there's a strong shared prefix
       but no consistent extension.

    Pattern format always matches ``http`` and ``https``, with or
    without ``www.`` — matches the conventions used elsewhere in
    ``sources.yaml``.
    """
    if len(article_urls) < min_sample:
        return None

    parsed_homepage = urlparse(homepage_origin)
    host = parsed_homepage.netloc.lower().removeprefix("www.")
    if not host:
        return None
    host_pattern = re.escape(host)
    host_alt = rf"https?://(www\.)?{host_pattern}"

    paths = [urlparse(u).path for u in article_urls]
    paths = [p for p in paths if p]
    if len(paths) < min_sample:
        return None

    # Pattern 1: <prefix>-<digits>.html — the tagesschau / classic
    # CoreMedia convention.
    slug_id_re = re.compile(r"-\d+\.html$")
    if sum(1 for p in paths if slug_id_re.search(p)) / len(paths) >= 0.8:
        return f"^{host_alt}/[^?#]+-\\d+\\.html$"

    # Pattern 2: common first path segment + .html extension.
    first_segments = [p.lstrip("/").split("/", 1)[0] for p in paths]
    seg_counts: dict[str, int] = {}
    for seg in first_segments:
        if seg:
            seg_counts[seg] = seg_counts.get(seg, 0) + 1
    if seg_counts:
        most_common_seg, most_common_n = max(seg_counts.items(), key=lambda kv: kv[1])
        if most_common_n / len(paths) >= 0.8:
            html_share = (
                sum(1 for p in paths if p.lstrip("/").startswith(most_common_seg + "/") and p.lower().endswith(".html"))
                / most_common_n
            )
            seg_escaped = re.escape(most_common_seg)
            if html_share >= 0.8:
                return f"^{host_alt}/{seg_escaped}/[^?#]+\\.html$"
            return f"^{host_alt}/{seg_escaped}/[^?#]+$"

    # Pattern 3: when no clear segment dominates, accept any HTML page
    # under the host at depth ≥ 2 (very conservative — likely to be
    # imprecise; operator should review).
    if sum(1 for p in paths if p.lower().endswith(".html")) / len(paths) >= 0.8:
        return f"^{host_alt}/[^?#]+\\.html$"

    return None


def _validate_article_listing_page(
    body: str,
    page_url: str,
    *,
    min_article_links: int = 5,
) -> dict[str, Any]:
    """Determine whether a page actually contains a meaningful list of
    article links — used to reject false-positive html_sitemap and
    archive_index candidates that return HTTP 200 + HTML but no real
    article content (the bundesregierung ``?datum=...`` failure mode).

    Returns ``{is_listing: bool, article_urls: list[str], reason: str}``.
    ``article_urls`` carries the extracted candidates (used downstream
    for pattern inference + operator-visible sample).
    """
    article_urls = _extract_article_url_candidates(body, page_url, self_url=page_url)
    if len(article_urls) < min_article_links:
        return {
            "is_listing": False,
            "article_urls": article_urls,
            "reason": f"only {len(article_urls)} article-shaped link(s) "
            f"found (threshold {min_article_links}) — likely a "
            "navigation page, not an article listing.",
        }
    return {
        "is_listing": True,
        "article_urls": article_urls,
        "reason": f"{len(article_urls)} article-shaped links found.",
    }


# CMS-family → conventional article URL pattern. When the strict
# inference rejects its own candidate, we fall back to suggesting one
# of these based on the homepage's `<meta name="generator">` tag.
# Each template uses `{HOST}` as a placeholder for the publisher's
# escaped host string. Patterns are conservative — they target the
# CMS's canonical article-URL form, not every URL that happens to be
# generated by that CMS.
CMS_PATTERN_TEMPLATES: dict[str, list[tuple[str, str]]] = {
    # WordPress permalink default = /YYYY/MM/dd/slug/ (and variants
    # without day or month). Some sites use ?p=N "ugly" permalinks.
    "wordpress": [
        (
            "WordPress /YYYY/MM/DD/slug/ permalink",
            r"^https?://(www\.)?{HOST}/\d{{4}}/\d{{2}}/\d{{2}}/[^/]+/?$",
        ),
        (
            "WordPress /YYYY/MM/slug/ permalink",
            r"^https?://(www\.)?{HOST}/\d{{4}}/\d{{2}}/[^/]+/?$",
        ),
        (
            "WordPress ?p= ugly permalink",
            r"^https?://(www\.)?{HOST}/\?p=\d+$",
        ),
    ],
    # Drupal default content URL = /node/N. Some sites pretty-URL it
    # to /article/<slug> or similar — second pattern covers that.
    "drupal": [
        ("Drupal /node/N", r"^https?://(www\.)?{HOST}/node/\d+$"),
        ("Drupal /article/slug", r"^https?://(www\.)?{HOST}/article/[^/?#]+$"),
    ],
    # Joomla canonical = /index.php?option=com_content&...&id=N or
    # /<category>/<slug>-N.html under SEF.
    "joomla": [
        ("Joomla SEF /category/slug-N.html", r"^https?://(www\.)?{HOST}/[^?#]+-\d+\.html$"),
        ("Joomla com_content", r"^https?://(www\.)?{HOST}/index\.php\?.*id=\d+"),
    ],
    # TYPO3 — canonical varies wildly; common is /<path>/<slug>-N.html
    # or /breg-de/-style locale prefix as bundesregierung does.
    "typo3": [
        ("TYPO3 /<path>/<slug>-N.html", r"^https?://(www\.)?{HOST}/[^?#]+-\d+\.html$"),
        ("TYPO3 /<path>/<slug>-N (no extension)", r"^https?://(www\.)?{HOST}/[^?#]+-\d+/?$"),
    ],
    # CoreMedia (the German publisher backbone — used by tagesschau,
    # ARD, ZDF). Canonical = /<section>/<slug>-NNN.html.
    "coremedia": [
        ("CoreMedia /<section>/<slug>-NNN.html", r"^https?://(www\.)?{HOST}/[^?#]+-\d+\.html$"),
    ],
    # Ghost — canonical /<slug>/.
    "ghost": [
        ("Ghost /slug/", r"^https?://(www\.)?{HOST}/[^/?#]+/?$"),
    ],
    # Hugo / Jekyll / Wagtail — generic /YYYY/MM/DD/slug/ or
    # /YYYY/MM/slug.html static-site convention.
    "hugo": [
        ("Static /YYYY/MM/DD/slug/", r"^https?://(www\.)?{HOST}/\d{{4}}/\d{{2}}/\d{{2}}/[^/]+/?$"),
    ],
    "jekyll": [
        ("Static /YYYY/MM/DD/slug.html", r"^https?://(www\.)?{HOST}/\d{{4}}/\d{{2}}/\d{{2}}/[^/]+\.html$"),
    ],
    "wagtail": [
        ("Wagtail /<section>/<slug>/", r"^https?://(www\.)?{HOST}/[^/]+/[^/]+/?$"),
    ],
}


def cms_pattern_suggestions(
    cms_family: Optional[str],
    homepage_origin: str,
) -> list[tuple[str, str]]:
    """Return ``[(label, regex), ...]`` candidates the operator can
    pick from when the strict auto-inference rejects its own
    candidate. Empty list when no CMS hint is available.
    """
    if not cms_family:
        return []
    family = cms_family.lower()
    templates = CMS_PATTERN_TEMPLATES.get(family)
    if not templates:
        return []
    parsed = urlparse(homepage_origin)
    host = (parsed.netloc or "").lower().removeprefix("www.")
    if not host:
        return []
    escaped_host = re.escape(host)
    return [(label, tmpl.format(HOST=escaped_host)) for label, tmpl in templates]


def _extract_non_article_links(
    body: str,
    base_url: str,
) -> list[str]:
    """Return the links on a page that ARE excluded by the article-URL
    heuristic — i.e. navigation / footer / asset / cross-domain links.
    Used as the anti-match validation set for pattern inference: a
    well-formed `article_url_pattern` must reject ALL of these.
    """
    if not body:
        return []
    parsed_base = urlparse(base_url)
    if not parsed_base.netloc:
        return []
    base_host = parsed_base.netloc.lower().removeprefix("www.")
    self_canon = base_url.rstrip("/").lower()
    seen: set[str] = set()
    out: list[str] = []
    for match in _A_HREF_RE.finditer(body):
        href = match.group(1).strip()
        if not href or href.startswith(("mailto:", "tel:", "javascript:", "#")):
            continue
        absolute = urljoin(base_url, href)
        parsed = urlparse(absolute)
        if parsed.scheme not in ("http", "https"):
            continue
        canon = absolute.rstrip("/").lower()
        if canon in seen:
            continue
        seen.add(canon)
        # An entry is a "non-article link" if it would be filtered out
        # by _extract_article_url_candidates. Mirror those conditions:
        host = parsed.netloc.lower().removeprefix("www.")
        path = parsed.path or ""
        is_non_article = (
            host != base_host
            or canon == self_canon
            or not path
            or path == "/"
            or path.lower().endswith(_ASSET_EXTENSIONS)
            or path.count("/") < 2
        )
        if is_non_article:
            out.append(absolute)
    return out


def validate_inferred_pattern(
    pattern: str,
    *,
    article_urls: list[str],
    non_article_urls: list[str],
) -> dict[str, Any]:
    """Quantitatively validate an inferred ``article_url_pattern``
    regex against two URL sets:

    * ``article_urls`` — URLs the audit identified as article candidates.
      A good pattern matches **all** of these (recall = 100 %).
    * ``non_article_urls`` — navigation / footer / asset links on the
      same page. A good pattern matches **none** of these (false-
      positive rate = 0 %).

    Returns counts + example match / miss URLs so the operator can
    eyeball the validation result. The caller decides whether to write
    the pattern to YAML based on these numbers; the helper itself
    never writes.
    """
    try:
        compiled = re.compile(pattern)
    except re.error as exc:
        return {
            "valid": False,
            "reason": f"pattern does not compile: {exc}",
            "article_matched": 0,
            "article_total": len(article_urls),
            "non_article_matched": 0,
            "non_article_total": len(non_article_urls),
            "matched_articles": [],
            "missed_articles": list(article_urls)[:5],
            "false_positives": [],
        }

    matched_articles = [u for u in article_urls if compiled.match(u)]
    missed_articles = [u for u in article_urls if not compiled.match(u)]
    false_positives = [u for u in non_article_urls if compiled.match(u)]
    return {
        "valid": True,
        "reason": "pattern compiled and evaluated.",
        "article_matched": len(matched_articles),
        "article_total": len(article_urls),
        "non_article_matched": len(false_positives),
        "non_article_total": len(non_article_urls),
        "matched_articles": matched_articles[:5],
        "missed_articles": missed_articles[:5],
        "false_positives": false_positives[:5],
    }


def infer_safe_pattern(
    sample_articles: list[str],
    sample_non_articles: list[str],
    homepage_origin: str,
) -> tuple[Optional[str], dict[str, Any]]:
    """Try to derive an ``article_url_pattern`` regex that is safe to
    write into ``sources.yaml`` without operator review.

    A pattern is considered "safe" only when ALL of these hold:

    1. It's derived from the conservative pattern-1 heuristic (the
       narrow ``-NNN.html`` slug-with-numeric-id convention). Pattern-2
       and pattern-3 are deliberately NOT auto-applied — they're more
       permissive and have produced false positives in informal tests.
    2. It matches 100 % of the article-shaped sample URLs (no silent
       under-matching).
    3. It matches 0 % of the non-article-shaped sample URLs that the
       audit observed on the same page (no silent over-matching).

    Returns ``(pattern_or_None, diagnostic)``. When ``pattern`` is
    ``None`` the diagnostic explains why — used by the YAML applier to
    emit ``EDIT-ME-REGEX-...`` with a helpful comment instead.
    """
    # Run the inference but only accept its narrowest output. We re-run
    # internally so we can inspect what pattern would have been chosen
    # even if the conservative gate rejects it.
    pattern = _infer_article_url_pattern(sample_articles, homepage_origin)
    if not pattern:
        return None, {
            "rejected_reason": "no consistent slug-NNN.html pattern across the sample — "
            "publisher's URL convention is non-standard.",
            "sample_article_count": len(sample_articles),
        }

    # Only accept pattern-1 shape (slug-NNN.html). Pattern-2 / pattern-3
    # shapes are detectable by structure: pattern-1 always contains
    # the literal ``-\d+\.html`` token. Anything else → reject for
    # auto-apply.
    if r"-\d+\.html$" not in pattern:
        return None, {
            "rejected_reason": "inferred pattern is permissive (no slug-NNN.html "
            "convention detected) — would not auto-apply.",
            "inferred_pattern": pattern,
            "sample_article_count": len(sample_articles),
        }

    val = validate_inferred_pattern(
        pattern,
        article_urls=sample_articles,
        non_article_urls=sample_non_articles,
    )
    if not val["valid"]:
        return None, {
            "rejected_reason": f"pattern compile failed: {val['reason']}",
            "inferred_pattern": pattern,
        }
    if val["article_matched"] != val["article_total"]:
        return None, {
            "rejected_reason": f"pattern matches only {val['article_matched']}/"
            f"{val['article_total']} sampled articles (recall < 100 %).",
            "inferred_pattern": pattern,
            "diagnostic": val,
        }
    if val["non_article_matched"] > 0:
        return None, {
            "rejected_reason": f"pattern false-positively matches "
            f"{val['non_article_matched']}/{val['non_article_total']} "
            "non-article links on the same page.",
            "inferred_pattern": pattern,
            "diagnostic": val,
        }
    # 100 % recall, 0 % false-positives on the observed sample.
    return pattern, {
        "accepted": True,
        "diagnostic": val,
        "sample_article_count": len(sample_articles),
        "sample_non_article_count": len(sample_non_articles),
    }


def _detect_cms(homepage_html: str) -> Optional[str]:
    """Return the publisher's CMS family if its `<meta name="generator">`
    declares one. Useful as a heuristic hint for the operator."""
    if not homepage_html:
        return None
    match = _GENERATOR_META_RE.search(homepage_html[:8192])
    if not match:
        return None
    raw = match.group(1).strip()
    # Normalise common families.
    lowered = raw.lower()
    for family in ("wordpress", "drupal", "joomla", "typo3", "coremedia", "ghost", "hugo", "jekyll", "wagtail"):
        if family in lowered:
            return family
    return raw[:60]
