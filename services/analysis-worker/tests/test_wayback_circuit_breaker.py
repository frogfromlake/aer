"""WaybackCDXClient circuit breaker + session retry policy.

An unreachable Internet Archive must NOT add a per-article HTTP timeout to the
synchronous harmonisation path (which collapses worker throughput on a large
backlog). After N consecutive failures the breaker opens and lookups
short-circuit with no network call; a half-open probe closes it again on
recovery. The inline (ingest) session is fail-fast; the sweep session retries
patiently. Local rate-limit denials are not endpoint failures and must not
trip the breaker.
"""

from __future__ import annotations

import time

import requests

from internal.wayback.client import (
    CDXResult,
    STATUS_CIRCUIT_OPEN,
    STATUS_FAILED,
    STATUS_OK,
    STATUS_RATE_LIMITED,
    WaybackCDXClient,
    _build_session,
)


def _client(**overrides) -> WaybackCDXClient:
    defaults = dict(
        enabled=True,
        base_url="https://web.archive.org/cdx/search/cdx",
        timeout_seconds=5.0,
        rate_limit_per_second=1000.0,  # effectively no rate limiting in tests
        user_agent="AER-Test/1.0",
        cache=None,
    )
    defaults.update(overrides)
    return WaybackCDXClient(**defaults)


def test_circuit_opens_after_threshold_and_short_circuits() -> None:
    """After N consecutive failures, lookups stop hitting the network entirely."""
    client = _client(circuit_failure_threshold=3, circuit_reset_seconds=60.0)
    calls = {"n": 0}

    def boom(_url: str) -> CDXResult:
        calls["n"] += 1
        raise requests.exceptions.ConnectTimeout("archive unreachable")

    client._fetch_remote = boom  # type: ignore[assignment]

    # The first `threshold` lookups each attempt the network and fail.
    for _ in range(3):
        assert client.lookup("https://example.com/a").status == STATUS_FAILED
    assert calls["n"] == 3

    # Circuit is now OPEN: further lookups return the DISTINCT `circuit_open`
    # status immediately WITHOUT any network attempt — distinct from `failed`
    # (which means a call WAS attempted) so the observability table can tell
    # self-protection from a real IA outage, and neither is ever read as
    # "no edits".
    for _ in range(10):
        assert client.lookup("https://example.com/a").status == STATUS_CIRCUIT_OPEN
    assert calls["n"] == 3, "open circuit must not attempt the network"


def test_circuit_half_open_probe_recovers() -> None:
    """Once the cooldown elapses, a single successful probe closes the breaker."""
    client = _client(circuit_failure_threshold=2, circuit_reset_seconds=60.0)

    def boom(_url: str) -> CDXResult:
        raise requests.exceptions.ConnectTimeout("archive unreachable")

    client._fetch_remote = boom  # type: ignore[assignment]
    for _ in range(2):
        client.lookup("https://example.com/a")
    assert client._cb_open_until > 0.0  # breaker opened

    # Simulate the cooldown having elapsed → next call is a half-open probe.
    client._cb_open_until = time.monotonic() - 0.01
    client._fetch_remote = lambda _url: CDXResult(status=STATUS_OK)  # type: ignore[assignment]

    result = client.lookup("https://example.com/a")
    assert result.status == STATUS_OK
    assert client._cb_open_until == 0.0, "successful probe must close the breaker"
    assert client._cb_consecutive_failures == 0


def test_inline_session_fail_fast_sweep_patient() -> None:
    """ADR-036 — the inline (ingest) client must NOT add a long retry chain to
    queue-drain; the sweep client may retry patiently. Locked here so the
    fail-fast-inline decision can't silently regress to the 4x15s monster."""
    inline = _build_session("AER-Test/1.0", 1)
    sweep = _build_session("AER-Test/1.0", 3)
    assert inline.get_adapter("https://web.archive.org").max_retries.total == 1
    assert sweep.get_adapter("https://web.archive.org").max_retries.total == 3
    # max_retries=0 → a plain session (no retry adapter): one attempt, fail fast.
    fail_fast = _build_session("AER-Test/1.0", 0)
    assert fail_fast.get_adapter("https://web.archive.org").max_retries.total == 0


def test_rate_limit_denial_does_not_trip_breaker() -> None:
    """Local backpressure (rate-limit) is not an endpoint failure."""
    # rate_limit very low so the bucket denies after the initial burst token.
    client = _client(rate_limit_per_second=0.1, circuit_failure_threshold=2)
    network = {"n": 0}

    def ok(_url: str) -> CDXResult:
        network["n"] += 1
        return CDXResult(status=STATUS_OK)

    client._fetch_remote = ok  # type: ignore[assignment]
    # Drain the single burst token, then several denied calls.
    statuses = [client.lookup("https://example.com/a").status for _ in range(5)]
    # Rate-limit denials must not advance the breaker toward open.
    assert client._cb_open_until == 0.0
    # Denied calls report the DISTINCT `rate_limited` status — never `failed`
    # (not an endpoint problem) and never confusable with "no edits".
    assert STATUS_RATE_LIMITED in statuses
    assert STATUS_FAILED not in statuses
