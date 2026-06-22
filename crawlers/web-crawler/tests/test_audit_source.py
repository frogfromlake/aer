"""Tests for the Phase-122g audit-source-discovery CLI.

The audit CLI probes a candidate source's homepage and reports the
discovery channels the publisher exposes. Tests cover:

  * The HTTP probe helper handles 200 / non-200 / network-failure
    cleanly and never raises.
  * `audit_source(...)` enumerates HTML-sitemap + archive-index
    candidates from the configured pattern lists.
  * The YAML formatter produces an operator-pasteable block with
    placeholders where operator judgment is required (regex pattern,
    expected_floor).
  * Trafilatura degradation: when trafilatura is not importable, the
    report carries the documented `skipped — trafilatura not installed`
    marker rather than crashing.

Shared HTTP-mock helpers (`fake_resp`, `route_get`, `article_listing_html`)
live in :mod:`tests._audit_helpers`.
"""

from __future__ import annotations

import json
from datetime import datetime, timezone
from pathlib import Path
from unittest.mock import MagicMock, patch
from urllib.parse import urlparse

import pytest

import audit_source
import audit_core
import audit_reaudit

from tests._audit_helpers import article_listing_html, fake_resp, route_get


# ---------------------------------------------------------------------------
# Local fixtures / helpers shared across this file.
# ---------------------------------------------------------------------------

# The four-channel diff dict shape used by render/apply tests — every
# channel present so callers only override the one(s) under test.
_EMPTY_DIFF: dict[str, list[str]] = {
    "sitemap_urls": [],
    "rss_hint_urls": [],
    "html_sitemap_urls": [],
    "archive_index_urls": [],
}


def _diff(**channels: list[str]) -> dict[str, list[str]]:
    """Return a full four-channel diff dict with the given channels set."""
    return {**_EMPTY_DIFF, **channels}


def _write_sources_yaml(tmp_path: Path, body: str) -> Path:
    """Write a sources.yaml fixture and return its path."""
    yaml_path = tmp_path / "sources.yaml"
    yaml_path.write_text(body, encoding="utf-8")
    return yaml_path


def _reaudit_report(**overrides: object) -> dict:
    """Build a re-audit report dict with the standard empty-channel base.

    Tests override only the field(s) they exercise (typically
    `trafilatura_sitemaps_found`).
    """
    base = {
        "homepage": "https://x.test",
        "origin": "https://x.test",
        "trafilatura_sitemaps_found": [],
        "trafilatura_feeds_found": [],
        "rss_path_hits": [],
        "rss_catalogue_hits": [],
        "homepage_inline_feeds": [],
        "html_sitemap_candidates": [],
        "archive_index_candidates": [],
    }
    base.update(overrides)
    return base


# ---------------------------------------------------------------------------
# _probe_http
# ---------------------------------------------------------------------------


def test_probe_http_returns_status_and_body_size() -> None:
    http_get = MagicMock(return_value=fake_resp(status=200, body="<html>" + "x" * 5000))
    result = audit_source._probe_http("https://example.com/sitemap.html", http_get, 5.0)
    assert result["status"] == 200
    assert result["body_size"] > 1024
    assert result["has_html_tag"] is True


def test_probe_http_handles_non_200() -> None:
    http_get = MagicMock(return_value=fake_resp(status=404, body="not found"))
    result = audit_source._probe_http("https://example.com/missing", http_get, 5.0)
    assert result["status"] == 404


def test_probe_http_handles_network_failure() -> None:
    def boom(*_a, **_kw):
        raise ConnectionError("DNS lookup failed")

    result = audit_source._probe_http("https://example.com/", boom, 5.0)
    assert result["status"] == 0
    assert "ConnectionError" in result["error"]


# ---------------------------------------------------------------------------
# audit_source — candidate detection
# ---------------------------------------------------------------------------


def test_audit_source_invalid_url_raises() -> None:
    with pytest.raises(ValueError, match="invalid homepage URL"):
        audit_source.audit_source("not-a-url")


def test_audit_source_identifies_html_sitemap_hits() -> None:
    """A page that returns 200 + HTML + has ≥ 5 article-shaped links
    should appear in `html_sitemap_candidates`. Tightened from naive
    "any large body" after the bundesregierung false-positive bug."""
    # Padding to clear the > 1 KB body-size guard. Real HTML sitemap
    # pages are tens of kilobytes; the small fixture mimics that.
    article_listing = (
        "<html><body>"
        + "".join(
            f'<a href="https://www.tagesschau.de/news/story-{i}.html">x</a>'
            for i in range(15)
        )
        + "<!-- " + "x" * 1500 + " -->"
        + "</body></html>"
    )
    fake_get = route_get(
        {"infoservices/startseite-sitemap": fake_resp(status=200, body=article_listing)},
    )

    report = audit_source.audit_source(
        "https://www.tagesschau.de",
        http_get=fake_get,
    )
    html_hits = [
        h for h in report["html_sitemap_candidates"]
        if "infoservices/startseite-sitemap" in h.get("url", "")
    ]
    assert len(html_hits) == 1
    assert html_hits[0]["article_count"] >= 5


def test_audit_source_identifies_archive_index_hits() -> None:
    """An archive endpoint that returns DIFFERENT article-shaped links
    for different dates (real date walker) should be flagged as an
    archive_index candidate. Identical-body case is rejected (see
    `test_audit_source_rejects_static_archive_pages`)."""
    def fake_get(url, **kwargs):
        if "/archiv?datum=" in url:
            # Make the article set depend on the date. Pad so the body
            # clears the > 1 KB initial-probe size threshold.
            date_suffix = url.split("datum=", 1)[1]
            articles = "".join(
                f'<a href="https://www.tagesschau.de/news/{date_suffix}-{i}.html">x</a>'
                for i in range(10)
            )
            return fake_resp(
                status=200,
                body=f"<html><body>{articles}<!--{'x'*1500}--></body></html>",
                url=url,
            )
        return fake_resp(status=404, body="", url=url)

    report = audit_source.audit_source(
        "https://www.tagesschau.de",
        http_get=fake_get,
    )
    archive_hits = [
        h for h in report["archive_index_candidates"]
        if not h.get("skipped")
    ]
    assert len(archive_hits) >= 1
    assert "{date}" in archive_hits[0]["url_template"]
    assert archive_hits[0]["date_walker_overlap_ratio"] < 0.5


def test_audit_source_trafilatura_skipped_when_unimportable() -> None:
    """When trafilatura is not installed, the report records the
    documented marker rather than crashing."""
    with patch.object(audit_core, "_try_trafilatura_feeds", return_value=None), \
         patch.object(audit_core, "_try_trafilatura_sitemaps", return_value=None):
        report = audit_source.audit_source(
            "https://example.com",
            http_get=MagicMock(return_value=fake_resp(status=404)),
        )
    assert report["trafilatura_feeds_found"] == "skipped — trafilatura not installed"
    assert report["trafilatura_sitemaps_found"] == "skipped — trafilatura not installed"


# ---------------------------------------------------------------------------
# YAML suggestion + CLI surface
# ---------------------------------------------------------------------------


def test_yaml_suggestion_contains_operator_placeholders() -> None:
    """The YAML output flags every operator-judgment field with
    `<edit-me ...>` so a copy-paste deploy WITHOUT review is obviously
    broken (article_url_pattern is publisher-specific and must be
    derived from real article URLs)."""
    report = {
        "homepage": "https://x",
        "origin": "https://x",
        "trafilatura_sitemaps_found": ["https://x/sitemap.xml"],
        "trafilatura_feeds_found": ["https://x/article-1"],
        "html_sitemap_candidates": [
            {"url": "https://x/sitemap.html", "body_size": 50000, "content_type": "text/html"}
        ],
        "archive_index_candidates": [
            {"url_template": "/archiv?datum={date}", "sample_url": "https://x/archiv?datum=2026-05-12", "body_size": 30000}
        ],
    }
    out = audit_source._format_yaml_suggestion(report)
    assert "<edit-me" in out
    assert "expected_floor_per_run:" in out
    assert "Mediacloud" in out


def test_completeness_baseline_measures_observed_inventory() -> None:
    """Phase 148d (WP-007 §6) — the onboarding contract measures the
    publisher-declared inventory per channel so the underflow floor is a
    measured seed, not a hand-typed guess."""
    report = {
        "trafilatura_sitemaps_found": ["https://x/a", "https://x/b", "https://x/c"],  # 3
        "trafilatura_feeds_found": ["https://x/f1", "https://x/f2"],  # 2
        "html_sitemap_candidates": [
            {"url": "https://x/s.html", "article_count": 40},
            {"url": "https://x/skip", "skipped": True, "article_count": 999},
        ],
        "archive_index_candidates": [{"url_template": "/archiv", "article_count": 120}],
    }
    baseline = audit_core._compute_completeness_baseline(report)
    assert baseline["per_channel"] == {
        "sitemap": 3,
        "rss": 2,
        "html_sitemap": 40,  # skipped candidate excluded
        "archive_index": 120,
    }
    assert baseline["observed_total"] == 165
    assert baseline["suggested_floor_per_run"] == 82  # 165 // 2, conservative


def test_completeness_baseline_empty_when_nothing_observed() -> None:
    baseline = audit_core._compute_completeness_baseline(
        {"trafilatura_sitemaps_found": "skipped — trafilatura not installed"}
    )
    assert baseline["observed_total"] == 0
    assert baseline["suggested_floor_per_run"] == 0
    assert baseline["per_channel"] == {}


def test_yaml_suggestion_emits_measured_floor_when_baseline_present() -> None:
    """When the audit measured an inventory, the YAML carries a measured floor
    seed (not the bare <edit-me> placeholder)."""
    report = {
        "homepage": "https://x",
        "origin": "https://x",
        "trafilatura_sitemaps_found": ["https://x/a", "https://x/b"],
        "trafilatura_feeds_found": [],
        "completeness_baseline": {
            "per_channel": {"sitemap": 200},
            "observed_total": 200,
            "suggested_floor_per_run": 100,
        },
    }
    out = audit_source._format_yaml_suggestion(report)
    assert "expected_floor_per_run: 100" in out
    assert "measured at audit" in out


def test_cli_emits_yaml_by_default(capsys) -> None:
    with patch.object(audit_source, "audit_source") as fake_audit:
        fake_audit.return_value = {
            "homepage": "https://x",
            "origin": "https://x",
            "trafilatura_sitemaps_found": [],
            "trafilatura_feeds_found": [],
            "html_sitemap_candidates": [],
            "archive_index_candidates": [],
        }
        rc = audit_source.cli(["https://x"])
    assert rc == 0
    out = capsys.readouterr().out
    assert "discovery:" in out
    assert "sitemap_urls" in out


def test_cli_json_mode_emits_raw_report(capsys) -> None:
    with patch.object(audit_source, "audit_source") as fake_audit:
        fake_audit.return_value = {
            "homepage": "https://x",
            "trafilatura_sitemaps_found": ["a", "b"],
        }
        rc = audit_source.cli(["https://x", "--json"])
    assert rc == 0
    parsed = json.loads(capsys.readouterr().out)
    assert parsed["homepage"] == "https://x"
    assert parsed["trafilatura_sitemaps_found"] == ["a", "b"]


def test_cli_rejects_invalid_url(capsys) -> None:
    rc = audit_source.cli(["not-a-url"])
    assert rc == 2
    err = capsys.readouterr().err
    assert "invalid homepage URL" in err


# ---------------------------------------------------------------------------
# Phase 122g re-audit / diff / write-back tests.
# ---------------------------------------------------------------------------


def test_extract_discovered_urls_rolls_up_all_channels() -> None:
    report = {
        "origin": "https://x.test",
        "trafilatura_sitemaps_found": ["https://x.test/sitemap.xml"],
        "trafilatura_feeds_found": [],
        "rss_path_hits": [{"url": "https://x.test/feed.xml"}],
        "rss_catalogue_hits": [
            {"discovered_feeds": ["https://x.test/news/feed1.xml",
                                  "https://x.test/news/feed2.xml"]},
        ],
        "homepage_inline_feeds": ["https://x.test/atom.xml"],
        "html_sitemap_candidates": [{"url": "https://x.test/sitemap.html"}],
        "archive_index_candidates": [{"url_template": "/archiv?datum={date}"}],
    }
    rolled = audit_source.extract_discovered_urls(report)
    assert rolled["sitemap_urls"] == ["https://x.test/sitemap.xml"]
    assert rolled["rss_hint_urls"] == [
        "https://x.test/atom.xml",
        "https://x.test/feed.xml",
        "https://x.test/news/feed1.xml",
        "https://x.test/news/feed2.xml",
    ]
    assert rolled["html_sitemap_urls"] == ["https://x.test/sitemap.html"]
    assert rolled["archive_index_urls"] == [
        "https://x.test/archiv?datum={date}",
    ]


def test_diff_against_configured_reports_only_additions() -> None:
    discovered = _diff(
        sitemap_urls=["https://x.test/sitemap-a.xml", "https://x.test/sitemap-b.xml"],
        rss_hint_urls=["https://x.test/feed-1.xml", "https://x.test/feed-2.xml"],
    )
    configured = {
        "sitemap_urls": ["https://x.test/sitemap-a.xml"],
        "rss_hint_urls": ["https://x.test/feed-1.xml"],
    }
    diff = audit_source.diff_against_configured(discovered, configured)
    assert diff["sitemap_urls"] == ["https://x.test/sitemap-b.xml"]
    assert diff["rss_hint_urls"] == ["https://x.test/feed-2.xml"]


def test_diff_never_reports_removals() -> None:
    """Operator-configured URLs absent from the audit MUST NOT appear in
    the diff — disappearance is a methodological event, not a routine
    maintenance trigger."""
    discovered = _diff()
    configured = {
        "sitemap_urls": ["https://x.test/retired-sitemap.xml"],
        "rss_hint_urls": ["https://x.test/retired-feed.xml"],
    }
    diff = audit_source.diff_against_configured(discovered, configured)
    assert all(not urls for urls in diff.values())


def test_diff_canonicalises_trailing_slash_and_case() -> None:
    discovered = _diff(
        sitemap_urls=["https://x.test/sitemap.xml/"],
        rss_hint_urls=["HTTPS://X.TEST/Feed.XML"],
    )
    configured = {
        "sitemap_urls": ["https://x.test/sitemap.xml"],
        "rss_hint_urls": ["https://x.test/feed.xml"],
    }
    diff = audit_source.diff_against_configured(discovered, configured)
    assert diff["sitemap_urls"] == []
    assert diff["rss_hint_urls"] == []


def test_diff_archive_index_only_when_unconfigured() -> None:
    """If a source already has an `archive_index:` block, the audit
    does NOT propose a replacement — operator intent overrides
    auto-discovery on this one."""
    discovered = _diff(archive_index_urls=["https://x.test/archiv?datum={date}"])
    configured_with_archive = {
        "archive_index": {"url_template": "https://x.test/other?date={date}"}
    }
    configured_without_archive: dict = {}
    assert audit_source.diff_against_configured(
        discovered, configured_with_archive
    )["archive_index_urls"] == []
    assert audit_source.diff_against_configured(
        discovered, configured_without_archive
    )["archive_index_urls"] == ["https://x.test/archiv?datum={date}"]


def test_apply_diff_to_yaml_adds_new_urls_and_preserves_comments(tmp_path) -> None:
    yaml_path = _write_sources_yaml(
        tmp_path,
        "# Probe-level header comment.\n"
        "probe:\n"
        "  time_window_days: 7\n"
        "sources:\n"
        "  - name: example\n"
        "    # Important methodological note about this source.\n"
        "    discovery:\n"
        "      sitemap_urls:\n"
        "        - https://x.test/sitemap-a.xml\n"
        "      rss_hint_urls:\n"
        "        - https://x.test/feed-1.xml\n",
    )
    diff = _diff(
        sitemap_urls=["https://x.test/sitemap-b.xml"],
        rss_hint_urls=["https://x.test/feed-2.xml"],
    )
    added = audit_source.apply_diff_to_yaml(yaml_path, "example", diff)
    assert added == {
        "sitemap_urls": 1,
        "rss_hint_urls": 1,
        "html_sitemap_urls": 0,
        "archive_index_urls": 0,
    }

    new_body = yaml_path.read_text(encoding="utf-8")
    # Comments preserved (the load-bearing reason for ruamel.yaml).
    assert "Probe-level header comment" in new_body
    assert "Important methodological note about this source" in new_body
    # New URLs appended.
    assert "https://x.test/sitemap-b.xml" in new_body
    assert "https://x.test/feed-2.xml" in new_body
    # Original URLs still present (no removals).
    assert "https://x.test/sitemap-a.xml" in new_body
    assert "https://x.test/feed-1.xml" in new_body
    # Backup file written.
    assert (yaml_path.with_suffix(yaml_path.suffix + ".bak")).exists()


def test_apply_diff_to_yaml_is_idempotent(tmp_path) -> None:
    """Re-applying a diff that's already merged into the YAML is a no-op
    (no duplicate URLs added)."""
    yaml_path = _write_sources_yaml(
        tmp_path,
        "sources:\n"
        "  - name: example\n"
        "    discovery:\n"
        "      rss_hint_urls:\n"
        "        - https://x.test/feed.xml\n",
    )
    diff = _diff(rss_hint_urls=["https://x.test/feed.xml"])
    added = audit_source.apply_diff_to_yaml(yaml_path, "example", diff)
    assert added["rss_hint_urls"] == 0
    body = yaml_path.read_text(encoding="utf-8")
    # Exactly one occurrence.
    assert body.count("https://x.test/feed.xml") == 1


def test_apply_diff_to_yaml_html_sitemap_inserts_edit_me_pattern(tmp_path) -> None:
    """html_sitemap entries get a placeholder article_url_pattern the
    operator MUST replace — it's intentionally not a valid regex so the
    crawler will refuse to ingest the channel until edited."""
    yaml_path = _write_sources_yaml(
        tmp_path,
        "sources:\n"
        "  - name: example\n"
        "    discovery: {}\n",
    )
    diff = _diff(html_sitemap_urls=["https://x.test/sitemap.html"])
    audit_source.apply_diff_to_yaml(yaml_path, "example", diff)
    body = yaml_path.read_text(encoding="utf-8")
    assert "https://x.test/sitemap.html" in body
    assert "EDIT-ME-REGEX-MATCHING-ARTICLE-URLS" in body


def test_apply_diff_to_yaml_archive_index_block(tmp_path) -> None:
    yaml_path = _write_sources_yaml(
        tmp_path,
        "sources:\n"
        "  - name: example\n"
        "    discovery: {}\n",
    )
    diff = _diff(archive_index_urls=["https://x.test/archiv?datum={date}"])
    audit_source.apply_diff_to_yaml(yaml_path, "example", diff)
    body = yaml_path.read_text(encoding="utf-8")
    assert "archive_index:" in body
    assert "url_template" in body
    assert "EDIT-ME-REGEX-MATCHING-ARTICLE-URLS" in body


def test_apply_diff_to_yaml_unknown_source_raises(tmp_path) -> None:
    yaml_path = _write_sources_yaml(
        tmp_path,
        "sources:\n"
        "  - name: example\n"
        "    discovery: {}\n",
    )
    with pytest.raises(ValueError, match="not found"):
        audit_source.apply_diff_to_yaml(yaml_path, "does-not-exist", _diff())


# ---------------------------------------------------------------------------
# _prompt_yes_no — input handling
# ---------------------------------------------------------------------------


@pytest.mark.parametrize(
    ("stdin", "expected"),
    [
        pytest.param("", False, id="empty-applies-default-no"),
        pytest.param("j", True, id="german-yes"),
        pytest.param("Y", True, id="uppercase-y"),
    ],
)
def test_prompt_yes_no_input_handling(monkeypatch, stdin: str, expected: bool) -> None:
    monkeypatch.setattr("builtins.input", lambda _prompt: stdin)
    assert audit_source._prompt_yes_no("?", default_no=True) is expected


def test_prompt_yes_no_handles_closed_stdin(monkeypatch) -> None:
    def raise_eof(_prompt):
        raise EOFError
    monkeypatch.setattr("builtins.input", raise_eof)
    # Closed stdin: applies the default (no when default_no=True).
    assert audit_source._prompt_yes_no("?", default_no=True) is False


# ---------------------------------------------------------------------------
# render_diff
# ---------------------------------------------------------------------------


def test_render_diff_empty_says_no_new_surfaces() -> None:
    out = audit_source.render_diff(_diff(), color=False)
    assert "no new surfaces" in out


def test_render_diff_lists_each_channel_addition() -> None:
    out = audit_source.render_diff(
        _diff(
            sitemap_urls=["https://x.test/a.xml"],
            rss_hint_urls=["https://x.test/b.xml"],
        ),
        color=False,
    )
    assert "sitemap_urls: 1 new" in out
    assert "rss_hint_urls: 1 new" in out
    assert "+ https://x.test/a.xml" in out
    assert "+ https://x.test/b.xml" in out


# ---------------------------------------------------------------------------
# Feed-link extraction + CMS detection
# ---------------------------------------------------------------------------


def test_extract_feed_links_from_catalogue_finds_link_alternate() -> None:
    html = """
    <html><head>
      <link rel="alternate" type="application/rss+xml" href="/news/rss.xml">
      <link rel="alternate" type="application/atom+xml" href="https://x.test/atom.xml">
    </head><body>
      <a href="/feeds/breg-de/1151244/feed.xml">Pressemitteilungen</a>
      <a href="/about">unrelated link</a>
    </body></html>
    """
    feeds = audit_source._extract_feed_links_from_catalogue(html, "https://x.test/")
    assert "https://x.test/news/rss.xml" in feeds
    assert "https://x.test/atom.xml" in feeds
    assert "https://x.test/feeds/breg-de/1151244/feed.xml" in feeds


@pytest.mark.parametrize(
    ("generator_meta", "expected"),
    [
        pytest.param('<meta name="generator" content="WordPress 6.5"/>', "wordpress",
                     id="known-cms-wordpress"),
        pytest.param('<meta name="generator" content="CustomCMS v1.0"/>', "CustomCMS v1.0",
                     id="unknown-cms-truncated-string"),
        pytest.param("<html><body>nothing</body></html>", None,
                     id="no-generator-meta"),
    ],
)
def test_detect_cms(generator_meta: str, expected: str | None) -> None:
    assert audit_source._detect_cms(generator_meta) == expected


# ---------------------------------------------------------------------------
# _run_reaudit — operator workflow outcomes
# ---------------------------------------------------------------------------


def test_run_reaudit_declined_returns_zero(tmp_path, monkeypatch) -> None:
    """Operator answering 'n' to the y/N prompt is a valid workflow
    outcome, not an error — exit code MUST be 0 so make doesn't fail."""
    yaml_path = _write_sources_yaml(
        tmp_path,
        "sources:\n"
        "  - name: example\n"
        "    discovery:\n"
        "      sitemap_urls: []\n",
    )
    fake_report = _reaudit_report(trafilatura_sitemaps_found=["https://x.test/new.xml"])
    monkeypatch.setattr("builtins.input", lambda _prompt: "n")
    original_body = yaml_path.read_text(encoding="utf-8")
    with patch.object(audit_reaudit, "audit_source", return_value=fake_report):
        rc = audit_source._run_reaudit(
            yaml_path=yaml_path,
            source_name="example",
            homepage="https://x.test",
            timeout=5.0,
            verbose=False,
            auto_yes=False,
            dry_run=False,
        )
    assert rc == 0
    assert yaml_path.read_text(encoding="utf-8") == original_body


def test_run_reaudit_no_changes_returns_zero(tmp_path, capsys) -> None:
    yaml_path = _write_sources_yaml(
        tmp_path,
        "sources:\n"
        "  - name: example\n"
        "    discovery:\n"
        "      sitemap_urls:\n"
        "        - https://x.test/sitemap.xml\n",
    )
    fake_report = _reaudit_report(
        trafilatura_sitemaps_found=["https://x.test/sitemap.xml"]
    )
    with patch.object(audit_reaudit, "audit_source", return_value=fake_report):
        rc = audit_source._run_reaudit(
            yaml_path=yaml_path,
            source_name="example",
            homepage="https://x.test",
            timeout=5.0,
            verbose=False,
            auto_yes=False,
            dry_run=False,
        )
    assert rc == 0


def test_run_reaudit_dry_run_does_not_write(tmp_path, capsys) -> None:
    yaml_path = _write_sources_yaml(
        tmp_path,
        "sources:\n"
        "  - name: example\n"
        "    discovery:\n"
        "      sitemap_urls: []\n",
    )
    fake_report = _reaudit_report(
        trafilatura_sitemaps_found=["https://x.test/new-sitemap.xml"]
    )
    original_body = yaml_path.read_text(encoding="utf-8")
    with patch.object(audit_reaudit, "audit_source", return_value=fake_report):
        rc = audit_source._run_reaudit(
            yaml_path=yaml_path,
            source_name="example",
            homepage="https://x.test",
            timeout=5.0,
            verbose=False,
            auto_yes=False,
            dry_run=True,
        )
    # Operator workflow outcome (no diff applied) is NOT an error.
    # Exit code 0 so `make audit-probe ARGS='--dry-run'` doesn't fail.
    assert rc == 0
    # File unchanged.
    assert yaml_path.read_text(encoding="utf-8") == original_body
    # No backup file written.
    assert not yaml_path.with_suffix(yaml_path.suffix + ".bak").exists()


def test_run_reaudit_auto_yes_applies_diff(tmp_path) -> None:
    yaml_path = _write_sources_yaml(
        tmp_path,
        "sources:\n"
        "  - name: example\n"
        "    discovery:\n"
        "      sitemap_urls: []\n",
    )
    fake_report = _reaudit_report(
        trafilatura_sitemaps_found=["https://x.test/new-sitemap.xml"]
    )
    with patch.object(audit_reaudit, "audit_source", return_value=fake_report):
        rc = audit_source._run_reaudit(
            yaml_path=yaml_path,
            source_name="example",
            homepage="https://x.test",
            timeout=5.0,
            verbose=False,
            auto_yes=True,
            dry_run=False,
        )
    assert rc == 0
    assert "https://x.test/new-sitemap.xml" in yaml_path.read_text(encoding="utf-8")


# ---------------------------------------------------------------------------
# Hardening tests — sanity checks that prevent false-positive archive_index
# / html_sitemap candidates (introduced after the live bundesregierung
# `?datum=` bug, 2026-05-15).
# ---------------------------------------------------------------------------


def test_extract_article_url_candidates_filters_assets_and_short_paths() -> None:
    html = """
    <html><body>
      <a href="/article/foo-123.html">good</a>
      <a href="/news/bar-456.html">good</a>
      <a href="/section/baz-789.html">good</a>
      <a href="/about">too shallow</a>
      <a href="https://other-site.com/article">cross-domain</a>
      <a href="/style.css">asset</a>
      <a href="mailto:x@y.z">mailto</a>
      <a href="#anchor">anchor</a>
    </body></html>
    """
    urls = audit_source._extract_article_url_candidates(
        html, "https://www.x.test/some-page"
    )
    assert any("foo-123.html" in u for u in urls)
    assert any("bar-456.html" in u for u in urls)
    assert any("baz-789.html" in u for u in urls)
    assert all("style.css" not in u for u in urls)
    assert all("other-site.com" not in u for u in urls)
    assert all("mailto" not in u for u in urls)


def test_extract_article_url_candidates_excludes_self_url() -> None:
    html = (
        '<html><body><a href="https://www.x.test/me">self</a>'
        '<a href="/article/foo-1.html">a</a>'
        '<a href="/article/bar-2.html">b</a></body></html>'
    )
    urls = audit_source._extract_article_url_candidates(
        html, "https://www.x.test/me", self_url="https://www.x.test/me"
    )
    assert all("/me" != urlparse(u).path for u in urls)


def test_validate_article_listing_page_rejects_thin_pages() -> None:
    """A page with only navigation links (no real article list) MUST
    be rejected as html_sitemap / archive_index candidate."""
    thin_html = (
        '<html><body><a href="/about">About</a>'
        '<a href="/contact">Contact</a></body></html>'
    )
    result = audit_source._validate_article_listing_page(
        thin_html, "https://www.x.test/sitemap"
    )
    assert result["is_listing"] is False
    assert "navigation page" in result["reason"]


def test_validate_article_listing_page_accepts_rich_pages() -> None:
    html = article_listing_html([f"/news/article-{i}.html" for i in range(10)])
    result = audit_source._validate_article_listing_page(
        html, "https://www.x.test/sitemap"
    )
    assert result["is_listing"] is True
    assert len(result["article_urls"]) >= 5


def test_verify_date_walker_rejects_same_content_for_different_dates() -> None:
    """Regression: bundesregierung's ?datum=... endpoint returns the
    SAME generic navigation page regardless of date. The verifier MUST
    reject the candidate as 'not a real date walker'."""
    static_html = article_listing_html([f"/section/page-{i}.html" for i in range(10)])
    # SAME body regardless of date.
    fake_get = route_get({}, default=fake_resp(status=200, body=static_html))

    today = datetime(2026, 5, 15, tzinfo=timezone.utc)
    result = audit_source._verify_date_walker(
        "/archiv?datum={date}",
        origin="https://www.x.test",
        http_get=fake_get,
        timeout=5.0,
        today=today,
    )
    assert result["is_walker"] is False
    assert result["overlap_ratio"] > 0.5
    assert "ignored" in result["reason"]


def test_verify_date_walker_accepts_genuinely_different_content() -> None:
    """A real date walker (like tagesschau's /archiv?datum=) returns
    different article lists for different dates. The verifier MUST
    accept it."""
    def fake_get(url, **_kw):
        date_suffix = url.split("=")[-1]
        # Return a date-specific set of article URLs.
        html = article_listing_html(
            [f"/news/{date_suffix}-article-{i}.html" for i in range(10)]
        )
        return fake_resp(status=200, body=html, url=url)

    today = datetime(2026, 5, 15, tzinfo=timezone.utc)
    result = audit_source._verify_date_walker(
        "/archiv?datum={date}",
        origin="https://www.x.test",
        http_get=fake_get,
        timeout=5.0,
        today=today,
    )
    assert result["is_walker"] is True
    assert result["overlap_ratio"] < 0.5
    assert len(result["today_articles"]) >= 5
    assert len(result["old_articles"]) >= 5


def test_audit_source_rejects_static_archive_pages(monkeypatch) -> None:
    """End-to-end: a publisher with a fake archive endpoint should NOT
    appear in archive_index_candidates after Phase 122g hardening."""
    static_html = (
        "<html><body>"
        + "".join(f'<a href="https://www.x.test/nav-{i}">nav</a>' for i in range(20))
        + "</body></html>"
    )
    # Same HTML for every URL (homepage + probes) so it parses but no
    # archive candidate's body varies by date.
    fake_get = route_get({}, default=fake_resp(status=200, body=static_html))

    # Avoid trafilatura side-effects.
    monkeypatch.setattr(audit_core, "_try_trafilatura_feeds", lambda _hp: [])
    monkeypatch.setattr(audit_core, "_try_trafilatura_sitemaps", lambda _hp: [])

    report = audit_source.audit_source(
        "https://www.x.test",
        http_get=fake_get,
    )
    # No archive_index candidates should survive — all return same body.
    accepted = [h for h in report["archive_index_candidates"]
                if not h.get("skipped")]
    assert accepted == []


def test_audit_source_accepts_real_date_walker(monkeypatch) -> None:
    """Symmetric to the rejection test: a publisher whose archive
    endpoint actually changes per date is kept."""
    def fake_get(url, **_kw):
        if "datum=" in url:
            # Vary by date — extract anything that follows datum=.
            date_part = url.split("datum=", 1)[1]
            articles = [f"/news/{date_part}-{i}.html" for i in range(10)]
        else:
            articles = []
        items = "".join(
            f'<a href="https://www.x.test{p}">item</a>' for p in articles
        )
        # Padding to clear the > 1 KB body-size pre-check.
        body = f"<html><body>{items}<!--{'x' * 1500}--></body></html>"
        return fake_resp(status=200, body=body, url=url)

    monkeypatch.setattr(audit_core, "_try_trafilatura_feeds", lambda _hp: [])
    monkeypatch.setattr(audit_core, "_try_trafilatura_sitemaps", lambda _hp: [])

    report = audit_source.audit_source(
        "https://www.x.test",
        http_get=fake_get,
    )
    accepted = [h for h in report["archive_index_candidates"]
                if not h.get("skipped")]
    # At least one archive pattern should pass the date-walker check.
    assert any("datum=" in h["url_template"] for h in accepted), \
        f"expected datum={{date}} pattern in accepted candidates, got: {accepted}"
