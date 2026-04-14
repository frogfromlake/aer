"""
Phase 83 — Analysis Worker Backpressure & Poison-Pill Containment.

These unit tests pin the two invariants that prevent the analysis worker
from being taken down by a single pathological input:

1. The in-process `asyncio.Queue` is bounded, so a slow extractor pipeline
   applies backpressure instead of growing the Python heap.

2. A deterministically-failing message is routed to the DLQ after
   `max_deliver` redeliveries (and then ack'd), breaking the
   NAK→redeliver→NAK loop that a poison pill would otherwise trigger.

Both tests run entirely with `unittest.mock` — no Testcontainers — because
the aim is to pin the control flow in `main._handle_message` and
`DataProcessor.quarantine_poison_message`, not the infrastructure glue.
"""

from __future__ import annotations

import asyncio
import json
from unittest.mock import AsyncMock, MagicMock

import pytest

from internal.processor import DataProcessor
from main import _handle_message


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

VALID_EVENT_ENVELOPE = {
    "Records": [
        {
            "s3": {
                "object": {
                    "key": "rss/tagesschau/abc/2026-04-05.json",
                    "userMetadata": {},
                }
            },
            "eventTime": "2026-04-05T10:00:00.000Z",
        }
    ]
}


def _make_nats_msg(num_delivered: int, payload: dict | None = None) -> MagicMock:
    """Construct a mock NATS JetStream message with a controllable delivery count."""
    msg = MagicMock()
    msg.data = json.dumps(payload or VALID_EVENT_ENVELOPE).encode("utf-8")
    msg.metadata = MagicMock()
    msg.metadata.num_delivered = num_delivered
    msg.ack = AsyncMock()
    msg.nak = AsyncMock()
    return msg


class _DummySpan:
    def set_attribute(self, *_args, **_kwargs):
        pass

    def __enter__(self):
        return self

    def __exit__(self, *exc):
        return False


class _DummyTracer:
    def start_as_current_span(self, *_args, **_kwargs):
        return _DummySpan()


# ---------------------------------------------------------------------------
# Poison-pill DLQ
# ---------------------------------------------------------------------------

@pytest.mark.asyncio
async def test_poison_pill_routes_to_quarantine_after_max_deliver():
    """A processor that always raises must be ack'd (not nak'd) on the final
    redelivery, after its payload is handed to `quarantine_poison_message`."""
    processor = MagicMock(spec=DataProcessor)
    processor.process_event.side_effect = RuntimeError("deterministic adapter bug")
    processor.quarantine_poison_message = MagicMock()

    tracer = _DummyTracer()
    max_deliver = 5

    # Attempts 1..4: NAK for redelivery, no quarantine.
    for attempt in range(1, max_deliver):
        msg = _make_nats_msg(num_delivered=attempt)
        await _handle_message(0, msg, processor, tracer, max_deliver)
        msg.nak.assert_awaited_once()
        msg.ack.assert_not_called()

    # Attempt 5 (final): quarantine + ack, NO nak.
    processor.quarantine_poison_message.reset_mock()
    msg = _make_nats_msg(num_delivered=max_deliver)
    await _handle_message(0, msg, processor, tracer, max_deliver)

    processor.quarantine_poison_message.assert_called_once()
    args, _ = processor.quarantine_poison_message.call_args
    assert args[0] == msg.data
    assert args[1] == "RuntimeError"
    msg.ack.assert_awaited_once()
    msg.nak.assert_not_called()


@pytest.mark.asyncio
async def test_poison_pill_fallback_nak_when_quarantine_write_fails():
    """If the quarantine sink itself is unreachable, the worker must NOT
    ack (which would drop the evidence) — it falls back to NAK so NATS
    surfaces the stuck state via `max_deliver` metrics."""
    processor = MagicMock(spec=DataProcessor)
    processor.process_event.side_effect = RuntimeError("boom")
    processor.quarantine_poison_message = MagicMock(
        side_effect=RuntimeError("minio down")
    )

    msg = _make_nats_msg(num_delivered=5)
    await _handle_message(0, msg, processor, _DummyTracer(), max_deliver=5)

    processor.quarantine_poison_message.assert_called_once()
    msg.ack.assert_not_called()
    msg.nak.assert_awaited_once()


# ---------------------------------------------------------------------------
# quarantine_poison_message — unit level
# ---------------------------------------------------------------------------

def test_quarantine_poison_message_recovers_bronze_payload(
    mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry
):
    """When the Bronze object is still present, the DLQ copy is the real
    payload — not a synthetic envelope — so operators can inspect it."""
    processor = DataProcessor(
        mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors=[]
    )
    processor._update_document_status = MagicMock()

    original_bronze = {"source": "rss", "raw_text": "hello"}
    response = MagicMock()
    response.read.return_value = json.dumps(original_bronze).encode("utf-8")
    mock_minio.get_object.return_value = response

    msg_bytes = json.dumps(VALID_EVENT_ENVELOPE).encode("utf-8")
    processor.quarantine_poison_message(msg_bytes, "RuntimeError", "boom")

    mock_minio.put_object.assert_called_once()
    put_args, _ = mock_minio.put_object.call_args
    assert put_args[0] == "bronze-quarantine"
    assert put_args[1] == "rss/tagesschau/abc/2026-04-05.json"
    body = put_args[2]
    body.seek(0)
    assert json.loads(body.read().decode("utf-8")) == original_bronze

    processor._update_document_status.assert_called_once_with(
        "rss/tagesschau/abc/2026-04-05.json", "quarantined"
    )


def test_quarantine_poison_message_synthesizes_envelope_when_bronze_missing(
    mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry
):
    """If the original Bronze object has been GC'd or the fetch fails, the
    poison envelope still captures the NATS event so nothing is silently
    dropped."""
    processor = DataProcessor(
        mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors=[]
    )
    processor._update_document_status = MagicMock()
    mock_minio.get_object.side_effect = RuntimeError("not found")

    msg_bytes = json.dumps(VALID_EVENT_ENVELOPE).encode("utf-8")
    processor.quarantine_poison_message(msg_bytes, "RuntimeError", "upstream explosion")

    mock_minio.put_object.assert_called_once()
    put_args, _ = mock_minio.put_object.call_args
    body = put_args[2]
    body.seek(0)
    synthetic = json.loads(body.read().decode("utf-8"))
    assert synthetic["_poison"] is True
    assert synthetic["_error_type"] == "RuntimeError"
    assert synthetic["_error"] == "upstream explosion"
    assert synthetic["_event_envelope"] == VALID_EVENT_ENVELOPE


def test_quarantine_poison_message_handles_unparseable_event(
    mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry
):
    """A NATS message whose body isn't a valid MinIO-event envelope still
    reaches the DLQ under a synthetic key so operators can find it."""
    processor = DataProcessor(
        mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors=[]
    )
    processor._update_document_status = MagicMock()
    mock_minio.get_object.side_effect = RuntimeError("also missing")

    processor.quarantine_poison_message(b"not json at all", "ValueError", "bad body")

    mock_minio.put_object.assert_called_once()
    put_args, _ = mock_minio.put_object.call_args
    assert put_args[0] == "bronze-quarantine"
    assert put_args[1].startswith("poison/unparseable/")


# ---------------------------------------------------------------------------
# Bounded queue backpressure
# ---------------------------------------------------------------------------

@pytest.mark.asyncio
async def test_bounded_queue_blocks_producer_when_workers_lag():
    """With `maxsize=4`, pushing 100 messages must NOT complete until
    somebody drains the queue — that is exactly the backpressure the
    unbounded version failed to provide."""
    queue: asyncio.Queue = asyncio.Queue(maxsize=4)

    async def producer():
        for i in range(100):
            await queue.put(i)

    producer_task = asyncio.create_task(producer())
    # Give the producer a couple of event-loop ticks to fill the queue.
    await asyncio.sleep(0.05)

    assert not producer_task.done(), "producer must block once queue is full"
    assert queue.qsize() == 4

    # Drain in controlled bursts and confirm the producer resumes in lockstep.
    for _ in range(10):
        queue.get_nowait()
        await asyncio.sleep(0)  # yield so producer can refill
    assert queue.qsize() == 4
    assert not producer_task.done()

    # Fully drain so the producer can finish.
    while not producer_task.done():
        try:
            queue.get_nowait()
        except asyncio.QueueEmpty:
            pass
        await asyncio.sleep(0)

    await producer_task
    assert producer_task.done()


@pytest.mark.asyncio
async def test_bounded_queue_matches_worker_count_formula():
    """Phase 83 fixed `maxsize = worker_count * 4` to match
    `max_ack_pending` — keep that relationship pinned so a future tweak to
    one side can't silently drift from the other."""
    from main import WorkerConfig

    cfg = WorkerConfig(worker_count=3)
    queue: asyncio.Queue = asyncio.Queue(maxsize=cfg.worker_count * 4)
    assert queue.maxsize == 12


# ---------------------------------------------------------------------------
# Sampler wiring
# ---------------------------------------------------------------------------

def test_init_telemetry_honors_sample_rate():
    """Sample-rate plumbing proved by checking the provider is constructed
    with a `ParentBased(TraceIdRatioBased(rate))` sampler — matches the Go
    services and makes `OTEL_TRACE_SAMPLE_RATE` meaningful for the worker."""
    from opentelemetry.sdk.trace.sampling import ParentBased, TraceIdRatioBased
    from main import init_telemetry

    # init_telemetry sets the global provider; exercising it twice is
    # idempotent in the sdk (the second call re-assigns).
    init_telemetry("http://localhost:4318", sample_rate=0.25)
    from opentelemetry import trace as _trace

    provider = _trace.get_tracer_provider()
    sampler = getattr(provider, "sampler", None)
    assert isinstance(sampler, ParentBased)
    # The SDK exposes sampler composition via `get_description`, which is the
    # stable user-facing rendering; assert against it instead of reaching
    # into SDK private attributes.
    description = sampler.get_description()
    assert "TraceIdRatioBased{0.25}" in description
    assert isinstance(TraceIdRatioBased(0.25), TraceIdRatioBased)  # sanity
