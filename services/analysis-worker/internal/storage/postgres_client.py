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
    pool = ThreadedConnectionPool(
        minconn=1,
        maxconn=maxconn,
        host=os.getenv("POSTGRES_HOST", "localhost"),
        port=os.getenv("POSTGRES_PORT", "5432"),
        user=os.getenv("POSTGRES_USER", "aer_admin"),
        password=os.getenv("POSTGRES_PASSWORD", ""),
        database=os.getenv("POSTGRES_DB", "aer_metadata")
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
