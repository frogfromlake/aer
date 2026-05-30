"""Phase 123 hardening — deterministic tests for the two pipeline-stability fixes.

1. WaybackCDXClient circuit breaker — an unreachable Internet Archive must NOT
   add a per-article HTTP timeout to the synchronous harmonisation path (which
   collapses worker throughput on a large backlog). After N consecutive
   failures the breaker opens and lookups short-circuit with no network call;
   a half-open probe closes it again on recovery.

2. WikidataAliasIndex thread-safety — the worker resolves entities from an
   executor thread-pool; a single shared sqlite3 connection raises
   SQLITE_MISUSE ("bad parameter or other API misuse") under concurrency. The
   index must hand each thread its own read-only connection.
"""

from __future__ import annotations

import sqlite3
import threading
import time
from pathlib import Path

import requests

from internal.storage.wikidata_alias_index import WikidataAliasIndex
from internal.wayback.client import (
    CDXResult,
    STATUS_FAILED,
    STATUS_OK,
    WaybackCDXClient,
)


# ---------------------------------------------------------------------------
# Fix 1 — Wayback CDX circuit breaker
# ---------------------------------------------------------------------------
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

    # Circuit is now OPEN: further lookups return FAILED immediately WITHOUT
    # any network attempt — this is what keeps throughput up during an outage.
    for _ in range(10):
        assert client.lookup("https://example.com/a").status == STATUS_FAILED
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
    for _ in range(5):
        client.lookup("https://example.com/a")
    # Rate-limit denials must not advance the breaker toward open.
    assert client._cb_open_until == 0.0


# ---------------------------------------------------------------------------
# Fix 2 — WikidataAliasIndex per-thread connections
# ---------------------------------------------------------------------------
def _build_index(tmp_path: Path) -> WikidataAliasIndex:
    db_path = tmp_path / "aliases.db"
    conn = sqlite3.connect(str(db_path))
    conn.execute("CREATE TABLE aliases (alias TEXT PRIMARY KEY, qid TEXT, label TEXT)")
    conn.execute("INSERT INTO aliases VALUES ('merkel', 'Q567', 'Angela Merkel')")
    conn.commit()
    conn.close()
    return WikidataAliasIndex(str(db_path))


def test_per_thread_connections_are_distinct(tmp_path: Path) -> None:
    """Each thread must get its OWN sqlite connection (the anti-MISUSE guarantee)."""
    index = _build_index(tmp_path)
    assert index.resolve("merkel") == ("Q567", "Angela Merkel")

    ids: dict[str, int] = {}
    lock = threading.Lock()

    def worker(name: str) -> None:
        conn_id = id(index._conn())
        index.resolve("merkel")  # exercise a real query on this thread's conn
        with lock:
            ids[name] = conn_id

    threads = [threading.Thread(target=worker, args=(f"t{i}",)) for i in range(4)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()

    # 4 worker threads → 4 distinct connection objects.
    assert len(set(ids.values())) == 4
    # 4 workers + the constructing (test) thread = 5 tracked connections.
    assert len(index._all_conns) == 5

    index.close()
    assert index._all_conns == []
