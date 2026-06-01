"""
Enrichment completeness & periodic re-attempt (ADR-036).

A **general** background loop — NOT a wayback-specific one — that gives every
per-article enrichment a second/third/Nth chance to fill data that a
transient failure (external service down), a missing capability, or a later
extraction improvement left incomplete. Without it, such gaps are silent and
permanent until a manual re-crawl (the bundesregierung/elysee Wayback
regression). See ADR-036 and the "no silent permanent gaps" guardrail.

Design (reuses the `corpus.py` periodic-loop pattern):
  * A registry of `ReAttemptTask`s. Each task owns its ClickHouse pool access
    and any external client; the loop only orchestrates and times. A task's
    `run()` finds a bounded batch of articles whose enrichment is INCOMPLETE
    (read from that enrichment's completeness signal, e.g.
    `aer_gold.wayback_lookups`) and re-attempts each — re-calling the external
    service or re-extracting from archived Bronze — writing idempotently
    (ReplacingMergeTree(ingestion_version), so overlap/repeat is harmless).
  * `enrichment_reattempt_loop` runs the registry at worker boot and every
    `interval_seconds` thereafter, offloading the blocking ClickHouse work to a
    thread (`asyncio.to_thread`, like the corpus loops) and draining cleanly on
    SIGTERM.

Wayback is the first registered task (`internal.wayback_reattempt`); Phase 133
adds a re-extract-from-Bronze task for `custom_extractors` metadata. ANY new
external / degradable / later-improvable enrichment MUST register here.
"""

from __future__ import annotations

import asyncio
import os
import time
from collections import Counter
from dataclasses import dataclass, field
from typing import Protocol, Sequence

import structlog

logger = structlog.get_logger()


@dataclass
class ReAttemptConfig:
    """Tuneables for the periodic enrichment re-attempt loop (ADR-036)."""

    enabled: bool = field(
        default_factory=lambda: os.getenv("ENRICHMENT_REATTEMPT_ENABLED", "true").strip().lower()
        in {"1", "true", "yes", "on"}
    )
    # Default 30 min: revision/metadata gaps are slow-moving; a tighter cadence
    # only re-hammers an unreachable dependency. Each task's client keeps its
    # own circuit breaker / rate limit, which bounds per-tick external load.
    interval_seconds: float = field(
        default_factory=lambda: float(os.getenv("ENRICHMENT_REATTEMPT_INTERVAL_SECONDS", "1800"))
    )
    # Per-task per-tick cap on how many incomplete articles to re-attempt, so a
    # large backlog drains over several ticks instead of one long stall.
    batch_limit: int = field(
        default_factory=lambda: int(os.getenv("ENRICHMENT_REATTEMPT_BATCH_LIMIT", "200"))
    )
    # Wait before the FIRST tick so the loop does not compete with the boot-time
    # queue-drain burst for the ClickHouse pool / CDX rate limit (the corpus
    # loops do the same). The data it heals is not time-critical.
    initial_delay_seconds: float = field(
        default_factory=lambda: float(os.getenv("ENRICHMENT_REATTEMPT_INITIAL_DELAY_SECONDS", "120"))
    )


class ReAttemptTask(Protocol):
    """One enrichment's re-attempt contract. Implementations own their
    ClickHouse pool access + any external client. `run` is synchronous
    (blocking ClickHouse I/O) — the loop offloads it to a thread."""

    name: str

    def run(self, limit: int) -> Counter:
        """Find up to `limit` incomplete articles for this enrichment and
        re-attempt each, writing idempotently on success. Return a Counter of
        outcome labels (for the per-tick summary)."""
        ...


def run_reattempt_sweep(tasks: Sequence[ReAttemptTask], batch_limit: int) -> dict[str, Counter]:
    """Run every registered task once. Each task is isolated so one failure
    never aborts the rest — the completeness signal stays the source of truth
    for what still needs re-attempting. Returns per-task outcome counters."""
    summary: dict[str, Counter] = {}
    for task in tasks:
        try:
            summary[task.name] = task.run(batch_limit)
        except Exception as exc:
            logger.error(
                "Re-attempt task failed this tick; will retry next tick.",
                task=task.name,
                error=str(exc),
            )
            summary[task.name] = Counter({"task_failed": 1})
    return summary


async def enrichment_reattempt_loop(
    tasks: Sequence[ReAttemptTask],
    stop_event: asyncio.Event,
    config: ReAttemptConfig,
) -> None:
    """Background task: run the re-attempt registry at boot and every
    `interval_seconds`. Drains on `stop_event` (SIGTERM)."""
    if not config.enabled:
        logger.info("Enrichment re-attempt loop disabled (ENRICHMENT_REATTEMPT_ENABLED is off).")
        return
    if not tasks:
        logger.info("Enrichment re-attempt loop: no tasks registered; not starting.")
        return

    logger.info(
        "Enrichment re-attempt loop started (ADR-036).",
        tasks=[t.name for t in tasks],
        interval_seconds=config.interval_seconds,
        batch_limit=config.batch_limit,
        initial_delay_seconds=config.initial_delay_seconds,
    )
    # Let the boot-time queue-drain settle before the first tick.
    try:
        await asyncio.wait_for(stop_event.wait(), timeout=config.initial_delay_seconds)
        return  # stop fired during the initial delay
    except asyncio.TimeoutError:
        pass
    while not stop_event.is_set():
        started = time.monotonic()
        try:
            # Blocking ClickHouse + external I/O → off the event loop.
            summary = await asyncio.to_thread(run_reattempt_sweep, tasks, config.batch_limit)
            logger.info(
                "Enrichment re-attempt tick complete.",
                duration_seconds=round(time.monotonic() - started, 2),
                outcomes={name: dict(counter) for name, counter in summary.items()},
            )
        except Exception as exc:  # defensive — the loop must never die.
            logger.error("Enrichment re-attempt tick raised; continuing.", error=str(exc))
        try:
            await asyncio.wait_for(stop_event.wait(), timeout=config.interval_seconds)
        except asyncio.TimeoutError:
            pass
