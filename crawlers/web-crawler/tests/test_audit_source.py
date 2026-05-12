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
"""

from __future__ import annotations

from unittest.mock import MagicMock, patch

import pytest

import audit_source


def _fake_resp(status: int = 200, body: str = "<html><body>x</body></html>",
               content_type: str = "text/html") -> MagicMock:
    resp = MagicMock()
    resp.status_code = status
    resp.text = body
    resp.headers = {"Content-Type": content_type}
    resp.url = "https://example.com/probed"
    return resp


def test_probe_http_returns_status_and_body_size() -> None:
    http_get = MagicMock(return_value=_fake_resp(status=200, body="<html>" + "x" * 5000))
    result = audit_source._probe_http("https://example.com/sitemap.html", http_get, 5.0)
    assert result["status"] == 200
    assert result["body_size"] > 1024
    assert result["has_html_tag"] is True


def test_probe_http_handles_non_200() -> None:
    http_get = MagicMock(return_value=_fake_resp(status=404, body="not found"))
    result = audit_source._probe_http("https://example.com/missing", http_get, 5.0)
    assert result["status"] == 404


def test_probe_http_handles_network_failure() -> None:
    def boom(*_a, **_kw):
        raise ConnectionError("DNS lookup failed")

    result = audit_source._probe_http("https://example.com/", boom, 5.0)
    assert result["status"] == 0
    assert "ConnectionError" in result["error"]


def test_audit_source_invalid_url_raises() -> None:
    with pytest.raises(ValueError, match="invalid homepage URL"):
        audit_source.audit_source("not-a-url")


def test_audit_source_identifies_html_sitemap_hits() -> None:
    """When the HTML probe returns 200 + HTML + > 1 KB body for one of
    the candidate paths, it should appear in `html_sitemap_candidates`."""
    def fake_get(url, **kwargs):
        if "infoservices/startseite-sitemap" in url:
            return _fake_resp(status=200, body="<html>" + "x" * 50000)
        # All other paths 404.
        return _fake_resp(status=404, body="not found")

    report = audit_source.audit_source(
        "https://www.tagesschau.de",
        http_get=fake_get,
    )
    html_hits = [
        h for h in report["html_sitemap_candidates"]
        if "infoservices/startseite-sitemap" in h.get("url", "")
    ]
    assert len(html_hits) == 1
    assert html_hits[0]["body_size"] > 1024


def test_audit_source_identifies_archive_index_hits() -> None:
    """Today's date substituted into `/archiv?datum=...` returning 200
    + HTML should be flagged as an archive_index candidate."""
    def fake_get(url, **kwargs):
        if "/archiv?datum=" in url:
            return _fake_resp(status=200, body="<html>" + "x" * 30000)
        return _fake_resp(status=404, body="")

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


def test_audit_source_trafilatura_skipped_when_unimportable() -> None:
    """When trafilatura is not installed, the report records the
    documented marker rather than crashing."""
    with patch.object(audit_source, "_try_trafilatura_feeds", return_value=None), \
         patch.object(audit_source, "_try_trafilatura_sitemaps", return_value=None):
        report = audit_source.audit_source(
            "https://example.com",
            http_get=MagicMock(return_value=_fake_resp(status=404)),
        )
    assert report["trafilatura_feeds_found"] == "skipped — trafilatura not installed"
    assert report["trafilatura_sitemaps_found"] == "skipped — trafilatura not installed"


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
    import json
    parsed = json.loads(capsys.readouterr().out)
    assert parsed["homepage"] == "https://x"
    assert parsed["trafilatura_sitemaps_found"] == ["a", "b"]


def test_cli_rejects_invalid_url(capsys) -> None:
    rc = audit_source.cli(["not-a-url"])
    assert rc == 2
    err = capsys.readouterr().err
    assert "invalid homepage URL" in err
