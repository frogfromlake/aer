"""Shared HTTP-mock helpers for the audit-source CLI test suites.

The audit tests repeatedly stand up a fake ``requests``-style response
object and route a fake ``http_get(url, **kw)`` closure by URL-substring.
Centralising the two shapes here keeps the individual test files focused
on their distinct assertion intent rather than on boilerplate:

  * :func:`fake_resp` — the response builder (status / body / headers /
    url) every fake ``http_get`` returns.
  * :func:`route_get` — a factory returning a ``fake_get`` closure that
    dispatches by URL-substring against a mapping, falling back to a
    configurable default (a 404 by default).

Genuinely bespoke closures (date-dependent bodies, article-listing
generators) stay in their test files but build their responses via
:func:`fake_resp` so there is a single response shape across the suite.
"""

from __future__ import annotations

from collections.abc import Callable
from unittest.mock import MagicMock


def fake_resp(
    status: int = 200,
    body: str = "<html><body>x</body></html>",
    content_type: str = "text/html",
    url: str = "https://example.com/probed",
) -> MagicMock:
    """Build a ``requests``-style response mock used by every fake http_get."""
    resp = MagicMock()
    resp.status_code = status
    resp.text = body
    resp.headers = {"Content-Type": content_type}
    resp.url = url
    return resp


def route_get(
    mapping: dict[str, MagicMock],
    default: MagicMock | None = None,
) -> Callable[..., MagicMock]:
    """Return a ``fake_get(url, **kw)`` that dispatches by URL-substring.

    The first key in ``mapping`` whose substring appears in the requested
    URL wins; otherwise ``default`` is returned (a 404 ``fake_resp`` when
    no default is supplied). The returned response's ``url`` attribute is
    rewritten to the requested URL so callers that inspect ``resp.url``
    (e.g. the date-walker verifier) see the real request target.
    """
    fallback = default if default is not None else fake_resp(status=404, body="not found")

    def fake_get(url: str, **_kwargs: object) -> MagicMock:
        for needle, resp in mapping.items():
            if needle in url:
                resp.url = url
                return resp
        fallback.url = url
        return fallback

    return fake_get


def article_listing_html(article_paths: list[str], host: str = "x.test") -> str:
    """Build a minimal HTML page with a navigation block + article links."""
    nav = '<a href="/about">About</a><a href="/contact">Contact</a>'
    items = "".join(f'<a href="https://www.{host}{p}">item</a>' for p in article_paths)
    return f"<html><body>{nav}{items}</body></html>"
