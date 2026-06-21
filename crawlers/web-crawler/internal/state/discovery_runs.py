"""Postgres writer for Phase 122g discovery telemetry.

Records one row per ``(source_id, channel)`` per discovery pass in
``crawler_discovery_runs`` so the BFF + dashboard can surface per-source-
per-channel coverage. When the URL count for a source's discovery run
falls below the source's ``expected_floor_per_run``, increments the
matching row in ``crawler_discovery_alerts`` — a two-consecutive-run
threshold gates the actual alert state (transient hiccups do not fire).

Universal-core writer: future Twitter / Reddit / Mastodon / YouTube
crawlers feed the same row shape with their own ``channel`` values.
Cross-platform comparability of coverage requires no schema work per
new platform class.

Test seam: pass an alternative ``conn_factory`` to inject a fake
connection for unit tests. The production default uses the shared
:class:`internal.state.dedup.CrawlerState` connection pool.
"""

from __future__ import annotations

import logging
import uuid
from dataclasses import dataclass
from datetime import datetime
from typing import Iterable, Optional

logger = logging.getLogger(__name__)


@dataclass(frozen=True)
class DiscoveryRunRecord:
    """One row's worth of telemetry — one (source, channel) per crawl run."""

    source_id: int
    channel: str
    urls_discovered: int
    urls_after_dedup: int
    run_started_at: datetime
    run_completed_at: datetime


class DiscoveryRunsWriter:
    """Postgres writer for the Phase-122g discovery-telemetry tables.

    Wraps a :class:`psycopg2.pool.ThreadedConnectionPool` (the one already
    held by :class:`internal.state.dedup.CrawlerState`). Two write paths:

    * :meth:`record_run` — INSERT into ``crawler_discovery_runs``. One
      call per (source, channel) per crawl pass.
    * :meth:`evaluate_alerts` — after every source's discovery pass,
      compares this run's total URL count against the source's
      ``expected_floor_per_run``. If two consecutive runs have fallen
      below the floor, UPSERT a row into ``crawler_discovery_alerts``;
      if this run recovered, DELETE the row.
    """

    def __init__(self, pool):
        self._pool = pool

    # ------------------------------------------------------------------
    # crawler_discovery_runs
    # ------------------------------------------------------------------
    def record_run(self, record: DiscoveryRunRecord) -> uuid.UUID:
        """Insert one telemetry row; return the generated run_id."""
        run_id = uuid.uuid4()
        conn = self._pool.getconn()
        try:
            with conn:
                with conn.cursor() as cur:
                    cur.execute(
                        """
                        INSERT INTO crawler_discovery_runs
                            (run_id, source_id, channel, urls_discovered,
                             urls_after_dedup, run_started_at, run_completed_at)
                        VALUES (%s, %s, %s, %s, %s, %s, %s)
                        """,
                        (
                            str(run_id),
                            record.source_id,
                            record.channel,
                            record.urls_discovered,
                            record.urls_after_dedup,
                            record.run_started_at,
                            record.run_completed_at,
                        ),
                    )
        finally:
            self._pool.putconn(conn)
        return run_id

    def record_run_batch(self, records: Iterable[DiscoveryRunRecord]) -> list[uuid.UUID]:
        """Insert multiple telemetry rows in a single connection
        round-trip. Used at end-of-discovery so all per-channel rows for
        a source land atomically."""
        rows: list[tuple] = []
        ids: list[uuid.UUID] = []
        for rec in records:
            rid = uuid.uuid4()
            ids.append(rid)
            rows.append(
                (
                    str(rid),
                    rec.source_id,
                    rec.channel,
                    rec.urls_discovered,
                    rec.urls_after_dedup,
                    rec.run_started_at,
                    rec.run_completed_at,
                )
            )
        if not rows:
            return ids
        conn = self._pool.getconn()
        try:
            with conn:
                with conn.cursor() as cur:
                    cur.executemany(
                        """
                        INSERT INTO crawler_discovery_runs
                            (run_id, source_id, channel, urls_discovered,
                             urls_after_dedup, run_started_at, run_completed_at)
                        VALUES (%s, %s, %s, %s, %s, %s, %s)
                        """,
                        rows,
                    )
        finally:
            self._pool.putconn(conn)
        return ids

    # ------------------------------------------------------------------
    # crawler_discovery_alerts
    # ------------------------------------------------------------------
    def evaluate_alerts(
        self,
        source_id: int,
        expected_floor: Optional[int],
        urls_after_dedup_this_run: int,
        run_started_at: datetime,
    ) -> Optional[str]:
        """Compare this run's total URL count against the source's
        ``expected_floor_per_run`` and update the alerts table.

        Two-consecutive-runs gate: the alert state requires two runs in
        a row below the floor. A single below-floor run is treated as a
        transient observation. On recovery (first run back at or above
        the floor) the alert row is deleted.

        ``expected_floor=None`` is a no-op — sources that haven't
        declared a floor are not eligible for underflow alerting.

        Returns a short event tag for the structured-warning log line
        the caller emits at end-of-source:
          * ``"alerted"``     — alert fired this run (two-in-a-row hit)
          * ``"pending"``     — first below-floor run, awaiting second
          * ``"recovered"``   — alert row cleared this run
          * ``None``          — no change (above floor, no prior pending)
        """
        if expected_floor is None or expected_floor <= 0:
            return None

        below_floor = urls_after_dedup_this_run < expected_floor

        conn = self._pool.getconn()
        try:
            with conn:
                with conn.cursor() as cur:
                    cur.execute(
                        """
                        SELECT consecutive_runs, alert_type, first_observed_at
                          FROM crawler_discovery_alerts
                         WHERE source_id = %s
                           AND alert_type IN ('underflow_pending', 'underflow')
                        """,
                        (source_id,),
                    )
                    row = cur.fetchone()
                    prior_runs = row[0] if row else 0
                    prior_type = row[1] if row else None
                    # Preserve the genuine first-observed timestamp from the
                    # pending row when promoting to a fired alert. (SEC-080: the
                    # promote INSERT previously bound row[0] — the int run-count —
                    # into the first_observed_at TIMESTAMPTZ column, which raised a
                    # datatype mismatch swallowed by the broad except below, so the
                    # underflow alert never fired for any source.)
                    prior_first_observed = row[2] if row else None

                    if below_floor:
                        new_runs = (prior_runs or 0) + 1
                        if new_runs >= 2:
                            # Promote / refresh the underflow alert row.
                            cur.execute(
                                """
                                INSERT INTO crawler_discovery_alerts
                                    (source_id, alert_type, first_observed_at,
                                     last_observed_at, consecutive_runs,
                                     expected_floor, last_urls_observed)
                                VALUES (%s, 'underflow', %s, %s, %s, %s, %s)
                                ON CONFLICT (source_id, alert_type) DO UPDATE
                                SET last_observed_at = EXCLUDED.last_observed_at,
                                    consecutive_runs = EXCLUDED.consecutive_runs,
                                    last_urls_observed = EXCLUDED.last_urls_observed
                                """,
                                (
                                    source_id,
                                    prior_first_observed or run_started_at,
                                    run_started_at,
                                    new_runs,
                                    expected_floor,
                                    urls_after_dedup_this_run,
                                ),
                            )
                            # Clear the pending row if it was distinct.
                            cur.execute(
                                """
                                DELETE FROM crawler_discovery_alerts
                                 WHERE source_id = %s AND alert_type = 'underflow_pending'
                                """,
                                (source_id,),
                            )
                            return "alerted"
                        else:
                            # First below-floor run — record a pending
                            # marker without firing the alert.
                            cur.execute(
                                """
                                INSERT INTO crawler_discovery_alerts
                                    (source_id, alert_type, first_observed_at,
                                     last_observed_at, consecutive_runs,
                                     expected_floor, last_urls_observed)
                                VALUES (%s, 'underflow_pending', %s, %s, 1, %s, %s)
                                ON CONFLICT (source_id, alert_type) DO UPDATE
                                SET last_observed_at = EXCLUDED.last_observed_at,
                                    consecutive_runs = 1,
                                    last_urls_observed = EXCLUDED.last_urls_observed
                                """,
                                (
                                    source_id,
                                    run_started_at,
                                    run_started_at,
                                    expected_floor,
                                    urls_after_dedup_this_run,
                                ),
                            )
                            return "pending"
                    else:
                        # Recovery — clear any existing alert / pending row.
                        if prior_type:
                            cur.execute(
                                """
                                DELETE FROM crawler_discovery_alerts
                                 WHERE source_id = %s
                                   AND alert_type IN ('underflow', 'underflow_pending')
                                """,
                                (source_id,),
                            )
                            return "recovered"
                        return None
        except Exception as exc:
            logger.warning(
                "crawler_discovery_alerts evaluate failed for source=%s: %s",
                source_id,
                exc,
            )
            return None
        finally:
            self._pool.putconn(conn)
