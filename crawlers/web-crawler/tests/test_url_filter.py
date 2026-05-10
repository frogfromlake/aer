"""Phase 122e A22 / F-A22 — exclude_path_prefixes matching robustness.

Verifies the URL filter matches a configured prefix `/foo/bar/` against
both the trailing-slash form (`/foo/bar/`) and the no-trailing-slash
landing-page form (`/foo/bar`), and is case-insensitive on both sides,
while still respecting word boundaries (so `/foo/bar` does NOT match
`/foo/barber/...`).
"""

from __future__ import annotations

from internal.fetch.scrapy_spider import _passes_url_filter


def _filter(prefixes):
    return {"exclude_path_prefixes": list(prefixes)}


# ----- trailing-slash robustness (the F-A22 fix) ---------------------------


def test_prefix_with_trailing_slash_matches_landing_page_without_trailing_slash():
    """Iter-4 leaked URL: configured `/breg-de/service/archiv-bundesregierung/`
    must match `/breg-de/service/archiv-bundesregierung` (the landing page).
    """
    flt = _filter(["/breg-de/service/archiv-bundesregierung/"])
    assert not _passes_url_filter(
        "https://www.bundesregierung.de/breg-de/service/archiv-bundesregierung", flt
    )


def test_prefix_without_trailing_slash_matches_subpaths():
    """Configured `/breg-de/service/archiv-bundesregierung` (no trailing
    slash) still matches `/breg-de/service/archiv-bundesregierung/anything`.
    """
    flt = _filter(["/breg-de/service/archiv-bundesregierung"])
    assert not _passes_url_filter(
        "https://www.bundesregierung.de/breg-de/service/archiv-bundesregierung/alt-inhalte/foo-1",
        flt,
    )


def test_trailing_slash_prefix_does_NOT_match_word_boundary_violations():
    """`/foo/news/` (with trailing slash) is segment-prefix; must not
    match `/foo/newsletter/...` — boundary required.
    """
    flt = _filter(["/breg-de/news/"])
    assert _passes_url_filter("https://x/breg-de/newsletter/123.html", flt)


def test_no_trailing_slash_prefix_uses_raw_startswith():
    """`/foo/slug-` (no trailing slash) is a raw-startswith pattern.
    The operator chose this form deliberately — it filters slug-style
    URLs like `/foo/slug-12345` regardless of any boundary character.
    """
    flt = _filter(["/breg-de/news"])
    # Raw startswith — both `/breg-de/news` and `/breg-de/newsletter` match.
    assert not _passes_url_filter("https://x/breg-de/newsletter/123.html", flt)
    assert not _passes_url_filter("https://x/breg-de/news/123.html", flt)


# ----- existing semantics still hold ---------------------------------------


def test_prefix_drops_exact_match_path():
    flt = _filter(["/breg-de/suche/"])
    assert not _passes_url_filter("https://x/breg-de/suche/", flt)
    assert not _passes_url_filter("https://x/breg-de/suche", flt)


def test_prefix_drops_subpath_match():
    flt = _filter(["/breg-de/suche/"])
    assert not _passes_url_filter("https://x/breg-de/suche/foo-100.html", flt)


def test_unmatched_prefix_lets_url_through():
    flt = _filter(["/breg-en/"])
    assert _passes_url_filter("https://x/breg-de/aktuelles/foo-100.html", flt)


def test_extension_exclude_still_works():
    flt = {"exclude_extensions": ["jpg", "png"], "exclude_path_prefixes": []}
    assert not _passes_url_filter("https://x/banner.jpg", flt)
    assert _passes_url_filter("https://x/article-100.html", flt)


def test_case_insensitive_prefix_match():
    """Mixed-case URL paths should match a lower-case configured prefix."""
    flt = _filter(["/breg-de/Service/archiv-bundesregierung/"])
    # path is lowercased before comparison, prefix is lowercased — both match
    assert not _passes_url_filter(
        "https://x/breg-de/service/archiv-bundesregierung/foo", flt
    )


def test_iter4_noise_patterns_are_filtered():
    """Sanity check: the new A22 noise patterns from sources.yaml drop
    the actual iter-4 URLs they were added for.
    """
    flt = _filter([
        "/breg-de/richtige-wahl-aus-330-berufen-",
        "/breg-de/fakten-zur-regierungspolitik-",
        "/breg-de/bundesregierung/link-kopieren-",
    ])
    for url in [
        "https://www.bundesregierung.de/breg-de/richtige-wahl-aus-330-berufen-975874",
        "https://www.bundesregierung.de/breg-de/fakten-zur-regierungspolitik-975748",
        "https://www.bundesregierung.de/breg-de/bundesregierung/link-kopieren-2205244",
    ]:
        assert not _passes_url_filter(url, flt), f"should be filtered: {url}"
