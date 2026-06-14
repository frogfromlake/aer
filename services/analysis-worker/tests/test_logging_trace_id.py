"""Phase 154 — logs-to-traces correlation.

Verifies the structlog processor injects the active span's trace-id as a
32-char hex string (the form Tempo indexes), and is a no-op when no recording
span is active.
"""

from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider

from internal.logging import add_trace_id


def test_add_trace_id_injects_active_span_id():
    provider = TracerProvider()
    tracer = provider.get_tracer(__name__)

    with tracer.start_as_current_span("unit-span"):
        expected = format(
            trace.get_current_span().get_span_context().trace_id, "032x"
        )
        event = add_trace_id(None, "info", {"event": "hello"})

    assert event["trace_id"] == expected
    assert len(event["trace_id"]) == 32


def test_add_trace_id_noop_without_span():
    # No active span -> INVALID_SPAN with an all-zero, non-valid context.
    event = add_trace_id(None, "info", {"event": "hello"})
    assert "trace_id" not in event
