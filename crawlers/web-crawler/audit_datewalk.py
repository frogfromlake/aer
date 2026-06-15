"""Article-URL-pattern verification by date-walking — extracted from audit_source.py (Phase 141). Used by audit_core; depends on pattern + probe helpers."""

from __future__ import annotations

import logging
from typing import Any
from urllib.parse import urljoin

from audit_probe import _fetch_body
from audit_pattern import _extract_article_url_candidates, _extract_non_article_links

logger = logging.getLogger(__name__)


def _verify_date_walker(
    url_template: str,
    *,
    origin: str,
    http_get,
    timeout: float,
    today: Any,  # datetime
) -> dict[str, Any]:
    """Confirm an archive-index URL template actually behaves like a
    date walker: fetch today's date and a date one year earlier; the
    two pages must surface DIFFERENT article-link sets.

    A publisher whose ``?datum=YYYY-MM-DD`` parameter is silently
    ignored (bundesregierung is the canonical example) returns the
    same generic navigation page regardless of date. This check
    rejects such candidates BEFORE the audit ever proposes them as
    valid archive_index channels.

    Returns ``{is_walker: bool, today_url, old_url, today_articles,
    old_articles, overlap_ratio, reason}``.
    """
    from datetime import timedelta

    past = today - timedelta(days=365)

    def _resolve(when: Any) -> str:
        path = (
            url_template.replace("{date}", when.strftime("%Y-%m-%d"))
            .replace("{year}", when.strftime("%Y"))
            .replace("{month}", when.strftime("%m"))
            .replace("{day}", when.strftime("%d"))
        )
        return urljoin(origin + "/", path.lstrip("/"))

    today_url = _resolve(today)
    old_url = _resolve(past)

    today_status, today_body = _fetch_body(today_url, http_get, timeout)
    old_status, old_body = _fetch_body(old_url, http_get, timeout)

    if today_status != 200 or old_status != 200:
        return {
            "is_walker": False,
            "today_url": today_url,
            "old_url": old_url,
            "today_articles": [],
            "today_non_articles": [],
            "old_articles": [],
            "overlap_ratio": 1.0,
            "reason": f"one of the two probe dates returned non-200 (today={today_status}, past={old_status}).",
        }

    today_articles = _extract_article_url_candidates(today_body, today_url, self_url=today_url)
    today_non_articles = _extract_non_article_links(today_body, today_url)
    old_articles = _extract_article_url_candidates(old_body, old_url, self_url=old_url)

    if not today_articles and not old_articles:
        return {
            "is_walker": False,
            "today_url": today_url,
            "old_url": old_url,
            "today_non_articles": today_non_articles,
            "today_articles": [],
            "old_articles": [],
            "overlap_ratio": 1.0,
            "reason": "neither probe date surfaced any article-shaped links.",
        }

    today_set = {u.rstrip("/").lower() for u in today_articles}
    old_set = {u.rstrip("/").lower() for u in old_articles}
    if not today_set or not old_set:
        # One side surfaced articles, the other didn't — still suspicious
        # but we'll be lenient and treat as walker since at least one
        # date is yielding content.
        return {
            "is_walker": True,
            "today_url": today_url,
            "old_url": old_url,
            "today_articles": today_articles,
            "today_non_articles": today_non_articles,
            "old_articles": old_articles,
            "overlap_ratio": 0.0,
            "reason": "one date surfaced articles, the other didn't — asymmetric but plausible date walker.",
        }

    overlap = today_set & old_set
    # Overlap ratio: how much of the SMALLER set is shared. A genuine
    # date walker shares ~ 0 — articles from a year ago are not in
    # today's archive page. A fake walker (same page regardless of
    # date) shares ~ 1.0.
    overlap_ratio = len(overlap) / min(len(today_set), len(old_set))

    if overlap_ratio > 0.5:
        return {
            "is_walker": False,
            "today_url": today_url,
            "old_url": old_url,
            "today_articles": today_articles,
            "today_non_articles": today_non_articles,
            "old_articles": old_articles,
            "overlap_ratio": overlap_ratio,
            "reason": f"today's and 1y-old's article-link sets overlap by "
            f"{overlap_ratio:.0%} — the ?date= parameter is being "
            "ignored by the publisher (not a real date walker).",
        }
    return {
        "is_walker": True,
        "today_url": today_url,
        "old_url": old_url,
        "today_articles": today_articles,
        "old_articles": old_articles,
        "overlap_ratio": overlap_ratio,
        "reason": f"today vs. 1y-old overlap is {overlap_ratio:.0%} — "
        "distinct content per date, confirmed date walker.",
    }
