import os
import structlog
from psycopg2.pool import ThreadedConnectionPool
from tenacity import retry, wait_exponential, stop_after_delay

logger = structlog.get_logger()


def _log_retry(retry_state):
    """Callback to log a warning when an infrastructure connection fails."""
    logger.warning(
        "Infrastructure not ready, retrying...",
        target=retry_state.fn.__name__,
        attempt=retry_state.attempt_number,
        wait=retry_state.idle_for
    )


PG_POOL_HEADROOM = 2
"""Extra connections above worker_count reserved for ad-hoc callers such as
retention sweeps or health-probe paths that would otherwise contend with the
per-worker hot path. Two slots empirically cover both classes without
over-committing PostgreSQL's max_connections budget."""


@retry(
    wait=wait_exponential(multiplier=1, min=1, max=10),
    stop=stop_after_delay(30),
    before_sleep=_log_retry
)
def init_postgres(maxconn: int | None = None) -> ThreadedConnectionPool:
    """
    Initializes a thread-safe connection pool for PostgreSQL.

    The pool is sized symmetrically with the ClickHouse pool: the caller
    (main.py) passes worker_count + PG_POOL_HEADROOM so every NATS worker
    owns a dedicated connection instead of contending for a fixed slot.
    Falls back to 10 when no size is provided, preserving the legacy default
    for tests that do not thread the worker count through.
    """
    if maxconn is None:
        maxconn = 10
    if maxconn < 1:
        maxconn = 1
    statement_timeout_ms = os.getenv("WORKER_PG_STATEMENT_TIMEOUT_MS", "5000")
    pool = ThreadedConnectionPool(
        minconn=1,
        maxconn=maxconn,
        host=os.getenv("POSTGRES_HOST", "localhost"),
        port=os.getenv("POSTGRES_PORT", "5432"),
        user=os.getenv("POSTGRES_USER", "aer_admin"),
        password=os.getenv("POSTGRES_PASSWORD", ""),
        database=os.getenv("POSTGRES_DB", "aer_metadata"),
        options=f"-c statement_timeout={statement_timeout_ms}",
    )
    # Ping the database
    conn = pool.getconn()
    try:
        with conn.cursor() as cur:
            cur.execute("SELECT 1")
    finally:
        pool.putconn(conn)

    return pool


def get_document_status(pg_pool: ThreadedConnectionPool, obj_key: str) -> str | None:
    """Fetches the current processing status from PostgreSQL."""
    conn = pg_pool.getconn()
    try:
        with conn.cursor() as cur:
            cur.execute("SELECT status FROM documents WHERE bronze_object_key = %s", (obj_key,))
            res = cur.fetchone()
            return res[0] if res else None
    finally:
        pg_pool.putconn(conn)


def update_document_status(pg_pool: ThreadedConnectionPool, obj_key: str, status: str) -> None:
    """Updates the document status in PostgreSQL."""
    conn = pg_pool.getconn()
    try:
        with conn.cursor() as cur:
            cur.execute("UPDATE documents SET status = %s WHERE bronze_object_key = %s", (status, obj_key))
        conn.commit()
    finally:
        pg_pool.putconn(conn)


def release_document_claim(pg_pool: ThreadedConnectionPool, obj_key: str) -> bool:
    """Release a `processing` claim back to `uploaded` — A27 recovery.

    Called from the worker's exception handler when processing aborts
    mid-flight (anywhere between `try_claim_document` and the terminal
    `update_document_status` call). Without this, a worker exception
    would leave the document stuck in `processing` forever, and
    subsequent NATS redeliveries would see `status='processing'`,
    treat it as already-claimed, and skip — silently dropping the
    article.

    Compare-and-swap: only releases if status is currently `processing`
    (matching the claim we issued). If a terminal state was already
    set in the same transaction (e.g., quarantine called before the
    exception), the release is a no-op.

    Returns True iff a row was released.
    """
    conn = pg_pool.getconn()
    try:
        with conn.cursor() as cur:
            cur.execute(
                """
                UPDATE documents
                SET status = 'uploaded'
                WHERE bronze_object_key = %s
                  AND status = 'processing'
                RETURNING id
                """,
                (obj_key,),
            )
            released = cur.fetchone() is not None
        conn.commit()
        return released
    finally:
        pg_pool.putconn(conn)


def try_claim_document(pg_pool: ThreadedConnectionPool, obj_key: str) -> bool:
    """Atomic compare-and-swap claim — Phase 122e A27 / F-A27.

    Returns True iff this caller atomically transitioned the document
    from a non-terminal state (`pending`, `uploaded`, or `NULL`) to
    `processing`. Returns False if the document is already
    `processed` / `quarantined` / `processing` (another worker
    succeeded, was DLQed, or is currently working it).

    Replaces the previous SELECT-status-then-process pattern that
    permitted a race window: two concurrent NATS deliveries of the
    same MinIO event could both observe `status='uploaded'` (or NULL)
    and both proceed to insert into ClickHouse. Source-table
    deduplication caught the raw duplicate, but ClickHouse's
    AggregatingMergeTree MV trigger fires before the source-side
    dedup check on non-Replicated engines, so the MV state silently
    diverged from raw (each race produced one stale MV sample).

    Postgres' MVCC + ``UPDATE ... RETURNING`` semantics make this
    claim atomic: only one transaction sees the matching row, the
    losers see zero rows. The status-machine is therefore:

        pending / uploaded / NULL
              │
              │  try_claim_document  → True
              ▼
        processing
              │
              │  process succeeds         │  process fails
              ▼                           ▼
        processed                    quarantined

    A document already in `processing` returns False — the caller
    treats this identically to "already processed" and skips. (If
    that worker dies, the message will be redelivered after
    `ack_wait`; the new claimant is whichever worker wins the race
    on the next attempt. Stuck-in-`processing` recovery is out of
    scope here — Phase 83's max_deliver poison-pill path catches
    permanently-failing messages.)
    """
    conn = pg_pool.getconn()
    try:
        with conn.cursor() as cur:
            cur.execute(
                """
                UPDATE documents
                SET status = 'processing'
                WHERE bronze_object_key = %s
                  AND (status IS NULL OR status IN ('pending', 'uploaded'))
                RETURNING id
                """,
                (obj_key,),
            )
            won = cur.fetchone() is not None
        conn.commit()
        return won
    finally:
        pg_pool.putconn(conn)


def update_document_article_id(pg_pool: ThreadedConnectionPool, obj_key: str, article_id: str) -> None:
    """
    Persists the deterministic SHA-256 article_id alongside the documents row.

    The BFF article-detail endpoint (Phase 101) needs the inverse mapping
    (article_id → bronze_object_key) so an L5 Evidence request can resolve
    back to the Silver/Bronze object. The worker computes article_id during
    harmonization; this writes it to the row that ingestion-api created on
    upload, identified by the same bronze_object_key.
    """
    conn = pg_pool.getconn()
    try:
        with conn.cursor() as cur:
            cur.execute(
                "UPDATE documents SET article_id = %s WHERE bronze_object_key = %s",
                (article_id, obj_key),
            )
        conn.commit()
    finally:
        pg_pool.putconn(conn)


def get_source_classification(pg_pool: ThreadedConnectionPool, source_name: str) -> dict | None:
    """
    Fetches the most recent discourse classification for a source by name.

    Joins sources and source_classifications tables, returning the latest
    classification record. Returns None if the source has no classification.
    """
    conn = pg_pool.getconn()
    try:
        with conn.cursor() as cur:
            cur.execute(
                """
                SELECT sc.primary_function, sc.secondary_function, sc.emic_designation
                FROM source_classifications sc
                JOIN sources s ON sc.source_id = s.id
                WHERE s.name = %s
                ORDER BY sc.classification_date DESC
                LIMIT 1
                """,
                (source_name,)
            )
            row = cur.fetchone()
            if row is None:
                return None
            return {
                "primary_function": row[0],
                "secondary_function": row[1],
                "emic_designation": row[2],
            }
    finally:
        pg_pool.putconn(conn)
