"""Structured logging setup for the analysis worker.

Phase 154 — logs-to-traces correlation. Every worker log line carries the
active OpenTelemetry trace-id, so an operator can pivot from a log entry to the
matching distributed trace in Tempo (and vice-versa). This mirrors the Go
services, where the same `trace_id` field is attached to every access log.

`configure_logging` also gives the worker the dev/prod renderer switch the Go
`pkg/logger` already has: human-readable console output in development,
single-line JSON in production/staging.
"""

import logging
import os

import structlog
from opentelemetry import trace


def add_trace_id(_logger, _method_name, event_dict):
    """structlog processor that injects the active span's trace-id.

    The id is formatted as a 32-character lowercase hex string — the exact
    form Tempo indexes and the Go services emit — so a log line and its trace
    are correlatable by an identical `trace_id` value. It is a no-op when no
    recording span is active (e.g. background sweeps outside a request span).
    """
    span_context = trace.get_current_span().get_span_context()
    if span_context.is_valid:
        event_dict["trace_id"] = format(span_context.trace_id, "032x")
    return event_dict


def configure_logging(environment: str | None = None, level: str | None = None) -> None:
    """Configure the global structlog pipeline. Call once, early in main().

    Idempotent enough for tests (re-configuring simply replaces the chain).
    """
    environment = environment or os.getenv("APP_ENV", "development")
    level_name = (level or os.getenv("LOG_LEVEL", "INFO")).upper()
    level_no = getattr(logging, level_name, logging.INFO)

    renderer = (
        structlog.processors.JSONRenderer()
        if environment in ("production", "staging")
        else structlog.dev.ConsoleRenderer()
    )

    structlog.configure(
        processors=[
            structlog.contextvars.merge_contextvars,
            structlog.processors.add_log_level,
            add_trace_id,
            structlog.processors.TimeStamper(fmt="iso"),
            structlog.processors.StackInfoRenderer(),
            structlog.processors.format_exc_info,
            renderer,
        ],
        wrapper_class=structlog.make_filtering_bound_logger(level_no),
        cache_logger_on_first_use=True,
    )
