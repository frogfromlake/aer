"""Tests for the Bronze ingestion contract translator."""

from __future__ import annotations

from datetime import datetime, timezone

from internal.translate.contract import (
    FetchEnvelope,
    build_object_key,
    build_payload,
    filter_response_headers,
)


def test_build_object_key_is_deterministic() -> None:
    key_a = build_object_key("tagesschau", "https://www.tagesschau.de/inland/foo.html")
    key_b = build_object_key("tagesschau", "https://www.tagesschau.de/inland/foo.html")
    assert key_a == key_b
    assert key_a.startswith("web/tagesschau/")
    assert key_a.endswith(".json")


def test_build_object_key_canonical_url_changes_key() -> None:
    a = build_object_key("tagesschau", "https://x/a")
    b = build_object_key("tagesschau", "https://x/b")
    assert a != b


def test_filter_response_headers_strips_unknown() -> None:
    headers = {
        "Set-Cookie": "tracker=1",
        "ETag": "abc",
        "Last-Modified": "Mon, 05 May 2026 12:00:00 GMT",
        "Server": "nginx",
        "Content-Type": "text/html; charset=utf-8",
    }
    cleaned = filter_response_headers(headers)
    assert "set-cookie" not in cleaned
    assert "server" not in cleaned
    assert cleaned["etag"] == "abc"
    assert cleaned["content-type"].startswith("text/html")


def test_build_payload_minimum_envelope() -> None:
    envelope = FetchEnvelope(
        source="tagesschau",
        original_url="https://www.tagesschau.de/inland/x.html",
        canonical_url="https://www.tagesschau.de/inland/x.html",
        fetch_at=datetime(2026, 5, 8, 12, 0, tzinfo=timezone.utc),
        http_status=200,
        response_headers={"content-type": "text/html", "etag": "W/\"1\""},
        sitemap_lastmod=datetime(2026, 5, 7, 11, 0, tzinfo=timezone.utc),
        sitemap_section="inland",
    )
    object_key, payload = build_payload("<html><body>hi</body></html>", envelope)
    assert object_key.startswith("web/tagesschau/")
    assert payload["source_type"] == "web"
    assert payload["raw_html"].startswith("<html>")
    assert payload["canonical_url"] == envelope.canonical_url
    assert payload["fetch_at"] == "2026-05-08T12:00:00Z"
    assert payload["sitemap_lastmod"] == "2026-05-07T11:00:00Z"
    assert payload["sitemap_section"] == "inland"
    assert payload["etag"] == "W/\"1\""


def test_build_payload_rejects_empty_html() -> None:
    envelope = FetchEnvelope(
        source="x",
        original_url="https://x/",
        canonical_url="https://x/",
        fetch_at=datetime.now(tz=timezone.utc),
        http_status=200,
        response_headers={},
    )
    try:
        build_payload("", envelope)
    except ValueError:
        return
    raise AssertionError("expected ValueError on empty html")
