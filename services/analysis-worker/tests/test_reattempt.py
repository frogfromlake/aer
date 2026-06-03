"""ADR-036 — enrichment re-attempt framework + the Wayback re-attempt task.

Pins the invariant that turns silent permanent gaps into self-healing ones:
an incomplete per-article enrichment is found again on a later tick and
re-attempted, writing idempotently on success.
"""

from __future__ import annotations

from collections import Counter
from datetime import datetime, timezone

from internal.reattempt import run_reattempt_sweep
from internal.wayback.client import CDXResult, WaybackRevision
from internal.wayback_reattempt import WaybackReAttemptTask


# --- framework: run_reattempt_sweep -----------------------------------------


class _FakeTask:
    def __init__(self, name: str, outcomes: Counter | None = None, boom: bool = False) -> None:
        self.name = name
        self._outcomes = outcomes if outcomes is not None else Counter()
        self._boom = boom
        self.calls = 0

    def run(self, limit: int) -> Counter:
        self.calls += 1
        if self._boom:
            raise RuntimeError("task exploded")
        return self._outcomes


def test_sweep_aggregates_per_task_outcomes() -> None:
    t1 = _FakeTask("wayback", Counter({"ok": 3, "circuit_open": 2}))
    t2 = _FakeTask("metadata", Counter({"no_snapshots": 1}))
    summary = run_reattempt_sweep([t1, t2], batch_limit=50)
    assert summary["wayback"] == Counter({"ok": 3, "circuit_open": 2})
    assert summary["metadata"] == Counter({"no_snapshots": 1})


def test_sweep_isolates_a_failing_task() -> None:
    boom = _FakeTask("wayback", boom=True)
    ok = _FakeTask("metadata", Counter({"ok": 1}))
    summary = run_reattempt_sweep([boom, ok], batch_limit=50)
    # A task that raises is recorded but does NOT abort the others.
    assert summary["wayback"] == Counter({"task_failed": 1})
    assert summary["metadata"] == Counter({"ok": 1})
    assert ok.calls == 1


# --- Wayback re-attempt task -------------------------------------------------


class _FakeQueryResult:
    def __init__(self, rows):
        self.result_rows = rows


class _FakeRawClient:
    """Mimics the raw clickhouse-connect Client returned by `pool.getconn()`:
    it supports `query()` but its `insert()` — like the real driver — does NOT
    accept `deduplication_token`. Passing one is the bug this fixture pins:
    the re-attempt task must route writes through the pool wrapper, never the
    raw client."""

    def __init__(self, rows):
        self._rows = rows
        self.queries: list = []

    def query(self, sql, parameters=None):
        self.queries.append((sql, parameters))
        return _FakeQueryResult(self._rows)

    def insert(self, table, rows, column_names=None):
        raise AssertionError(
            "re-attempt wrote via the raw getconn client; it must use the "
            "pool wrapper (only the wrapper accepts deduplication_token)"
        )


class _FakePool:
    """Mimics `ClickHousePool`: `getconn()` yields the raw client (query only);
    `insert()` is the wrapper method that accepts + forwards
    `deduplication_token`."""

    def __init__(self, client):
        self._client = client
        self.gets = 0
        self.puts = 0
        self.inserts: list = []

    def getconn(self):
        self.gets += 1
        return self._client

    def putconn(self, c):
        self.puts += 1

    def insert(self, table, rows, column_names=None, deduplication_token=None):
        self.inserts.append({"table": table, "rows": rows})


class _FakeWayback:
    def __init__(self, result: CDXResult):
        self._result = result
        self.looked_up: list = []

    def lookup(self, url: str) -> CDXResult:
        self.looked_up.append(url)
        return self._result


_ROW = ("bundesregierung", "abc123", "https://www.bundesregierung.de/x-1", "circuit_open")


def test_find_incomplete_maps_rows_and_filters_in_sql() -> None:
    ch = _FakeRawClient([_ROW])
    task = WaybackReAttemptTask(_FakePool(ch), _FakeWayback(CDXResult(status="failed")))
    items = task._find_incomplete(ch, 200)
    assert items == [
        {
            "source": "bundesregierung",
            "article_id": "abc123",
            "canonical_url": "https://www.bundesregierung.de/x-1",
            "prev_status": "circuit_open",
            # `_find_incomplete` enriches each item with `published_at`
            # resolved from `aer_gold.metrics`. The fake client returns the
            # same single row for that second query, so its `max(timestamp)`
            # is keyed on "bundesregierung" (the row's first column) and the
            # lookup by article_id "abc123" misses → None. That is the real
            # fallback path (`_reattempt_one` then uses the lookup time).
            "published_at": None,
        }
    ]
    # The SQL excludes the completed statuses and requires a usable URL.
    sql = ch.queries[0][0]
    assert "NOT IN ('ok', 'no_snapshots')" in sql
    assert "canonical_url != ''" in sql


def test_reattempt_still_failing_records_status_only() -> None:
    pool = _FakePool(_FakeRawClient([]))
    task = WaybackReAttemptTask(pool, _FakeWayback(CDXResult(status="circuit_open")))
    item = {"source": "elysee", "article_id": "d", "canonical_url": "https://elysee.fr/y"}
    outcome = task._reattempt_one(item)
    assert outcome == "circuit_open"
    # Only the lookup-status row is written (no revisions) — but it IS written,
    # via the pool wrapper, so the gap stays visible and is re-attempted next tick.
    assert [i["table"] for i in pool.inserts] == ["aer_gold.wayback_lookups"]


def test_reattempt_recovered_writes_revisions_and_status() -> None:
    rev = WaybackRevision(
        snapshot_at=datetime(2026, 5, 20, 9, 0, tzinfo=timezone.utc),
        content_hash="deadbeef",
        archive_url="https://web.archive.org/web/20260520090000/https://x/y",
    )
    pool = _FakePool(_FakeRawClient([]))
    task = WaybackReAttemptTask(pool, _FakeWayback(CDXResult(status="ok", revisions=[rev])))
    item = {"source": "tagesschau", "article_id": "t1", "canonical_url": "https://x/y"}
    outcome = task._reattempt_one(item)
    assert outcome == "ok"
    tables = [i["table"] for i in pool.inserts]
    # Both the revision chain AND the refreshed lookup status are persisted
    # (through the pool wrapper).
    assert "aer_gold.wayback_lookups" in tables
    assert "aer_gold.article_revisions" in tables


def test_run_manages_pool_connection_and_aggregates() -> None:
    pool = _FakePool(_FakeRawClient([_ROW]))
    task = WaybackReAttemptTask(pool, _FakeWayback(CDXResult(status="circuit_open")))
    outcomes = task.run(200)
    assert outcomes == Counter({"circuit_open": 1})
    # The raw connection is acquired for the discovery query AND returned...
    assert pool.gets == 1 and pool.puts == 1
    # ...and the status refresh is written through the pool wrapper (never the
    # raw client, whose insert() would reject deduplication_token).
    assert [i["table"] for i in pool.inserts] == ["aer_gold.wayback_lookups"]
