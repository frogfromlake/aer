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


@retry(
    wait=wait_exponential(multiplier=1, min=1, max=10),
    stop=stop_after_delay(30),
    before_sleep=_log_retry
)
def init_postgres() -> ThreadedConnectionPool:
    """
    Initializes a thread-safe connection pool for PostgreSQL.
    """
    pool = ThreadedConnectionPool(
        minconn=1,
        maxconn=10,
        host=os.getenv("POSTGRES_HOST", "localhost"),
        port=os.getenv("POSTGRES_PORT", "5432"),
        user=os.getenv("POSTGRES_USER", "aer_admin"),
        password=os.getenv("POSTGRES_PASSWORD", "aer_secret"),
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
