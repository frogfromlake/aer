"""Unit tests for the worker's testable main.py helpers.

The NATS-lifecycle entrypoint `main()` itself is integration territory (excluded
per ADR-041); these cover the pure/boot-time helpers around it.
"""

from __future__ import annotations

import asyncio
import json
from types import SimpleNamespace
from unittest.mock import AsyncMock, MagicMock, patch
from urllib.parse import unquote

import pytest

from main import (
    _build_minio_event,
    _num_delivered,
    stale_processing_reaper_loop,
    validate_required_env,
)


# --- SEC-074: stale-processing reaper loop ----------------------------------


@pytest.mark.asyncio
async def test_reaper_loop_republishes_stale_and_refreshes_dlq_gauge():
    """One reaper tick: reclaim a stale doc, re-publish a synthetic event for
    it, and refresh dlq_size from the bucket count. The loop stops itself once
    the (mocked) publish fires, keeping the test deterministic."""
    stop_event = asyncio.Event()
    pg_pool = MagicMock()
    minio = MagicMock()
    minio.list_objects.return_value = [MagicMock(), MagicMock(), MagicMock()]

    async def fake_publish(_subject, _data):
        stop_event.set()  # end the loop after the first re-publish

    js = MagicMock()
    js.publish = AsyncMock(side_effect=fake_publish)

    with patch("main.reclaim_stale_processing", return_value=["web/x/1.json"]):
        await asyncio.wait_for(
            stale_processing_reaper_loop(
                pg_pool,
                minio,
                js,
                "aer.lake.bronze",
                stop_event,
                threshold_seconds=900,
                interval_seconds=0.01,
            ),
            timeout=5,
        )

    js.publish.assert_awaited_once()
    pub_args, _ = js.publish.call_args
    assert pub_args[0] == "aer.lake.bronze"
    parsed = json.loads(pub_args[1].decode("utf-8"))
    assert unquote(parsed["Records"][0]["s3"]["object"]["key"]) == "web/x/1.json"
    minio.list_objects.assert_called_once()  # dlq_size refreshed from the bucket


@pytest.mark.asyncio
async def test_reaper_loop_survives_a_failing_tick():
    """A transient error in reclaim must not crash the loop — it logs and keeps
    spinning until stopped (the loop must never take down the worker)."""
    stop_event = asyncio.Event()
    js = MagicMock()
    js.publish = AsyncMock()

    with patch("main.reclaim_stale_processing", side_effect=RuntimeError("pg blip")):
        task = asyncio.create_task(
            stale_processing_reaper_loop(
                MagicMock(),
                MagicMock(),
                js,
                "aer.lake.bronze",
                stop_event,
                threshold_seconds=900,
                interval_seconds=0.01,
            )
        )
        await asyncio.sleep(0.05)  # let several failing ticks elapse
        assert not task.done()  # an unhandled raise would have ended the task
        stop_event.set()  # set only from the event loop (thread-safe)
        await asyncio.wait_for(task, timeout=5)

    js.publish.assert_not_awaited()  # nothing reclaimed → nothing re-published


# --- SEC-074: synthetic re-publish envelope round-trips the handler parse ----


def test_build_minio_event_round_trips_handler_parse():
    """The reaper's synthetic event must parse back to the SAME object key the
    worker handler extracts — including keys with spaces/special chars that get
    URL-encoded on the wire."""
    obj_key = "web/franceinfo/2026 04 05/a+b&c.json"
    raw = _build_minio_event(obj_key, "2026-04-05T10:00:00+00:00")
    event = json.loads(raw.decode("utf-8"))
    record = event["Records"][0]
    # Mirror main._handle_message's extraction.
    assert unquote(record["s3"]["object"]["key"]) == obj_key
    assert record["eventTime"] == "2026-04-05T10:00:00+00:00"
    assert record["s3"]["object"]["userMetadata"] == {}


# --- validate_required_env (boot-time secret validation) ---------------------


def test_validate_required_env_passes_when_all_set(monkeypatch):
    monkeypatch.setenv("AER_TEST_REQUIRED", "value")
    validate_required_env(["AER_TEST_REQUIRED"])  # must not raise


def test_validate_required_env_raises_when_missing(monkeypatch):
    monkeypatch.delenv("AER_TEST_ABSENT", raising=False)
    with pytest.raises(SystemExit) as exc:
        validate_required_env(["AER_TEST_ABSENT"])
    assert "AER_TEST_ABSENT" in str(exc.value)


def test_validate_required_env_treats_whitespace_as_empty(monkeypatch):
    monkeypatch.setenv("AER_TEST_BLANK", "   ")
    with pytest.raises(SystemExit):
        validate_required_env(["AER_TEST_BLANK"])


def test_validate_required_env_reports_all_missing(monkeypatch):
    monkeypatch.delenv("AER_TEST_A", raising=False)
    monkeypatch.delenv("AER_TEST_B", raising=False)
    with pytest.raises(SystemExit) as exc:
        validate_required_env(["AER_TEST_A", "AER_TEST_B"])
    message = str(exc.value)
    assert "AER_TEST_A" in message and "AER_TEST_B" in message


# --- _num_delivered (JetStream delivery count, fail-safe) --------------------


def test_num_delivered_returns_count():
    msg = SimpleNamespace(metadata=SimpleNamespace(num_delivered=3))
    assert _num_delivered(msg) == 3


def test_num_delivered_zero_when_metadata_unavailable():
    assert _num_delivered(SimpleNamespace()) == 0
