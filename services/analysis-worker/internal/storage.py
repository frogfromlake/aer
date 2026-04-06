import os
import queue
import structlog
import clickhouse_connect
from minio import Minio
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
def init_minio() -> Minio:
    """
    Initializes the MinIO client with an exponential backoff retry mechanism.
    Forces a network call to ensure the service is actually reachable.
    """
    client = Minio(
        endpoint=os.getenv("MINIO_ENDPOINT", "localhost:9000"),
        access_key=os.getenv("MINIO_ROOT_USER", "minioadmin"),
        secret_key=os.getenv("MINIO_ROOT_PASSWORD", "minioadmin"),
        secure=False
    )
    # Force a network call to verify the connection
    client.list_buckets()
    return client

class ClickHousePool:
    """Thread-safe pool of ClickHouse clients. One client per concurrent worker."""

    def __init__(self, size: int):
        self._pool: queue.Queue[clickhouse_connect.driver.Client] = queue.Queue(maxsize=size)
        for _ in range(size):
            self._pool.put(self._create_client())

    @staticmethod
    def _create_client() -> clickhouse_connect.driver.Client:
        return clickhouse_connect.get_client(
            host=os.getenv("CLICKHOUSE_HOST", "localhost"),
            port=int(os.getenv("CLICKHOUSE_PORT", "8123")),
            username=os.getenv("CLICKHOUSE_USER", "default"),
            password=os.getenv("CLICKHOUSE_PASSWORD", ""),
            database=os.getenv("CLICKHOUSE_DB", "aer_gold"),
        )

    def getconn(self) -> clickhouse_connect.driver.Client:
        return self._pool.get()

    def putconn(self, client: clickhouse_connect.driver.Client) -> None:
        self._pool.put(client)

    def insert(self, table: str, rows: list, column_names: list[str]) -> None:
        client = self.getconn()
        try:
            client.insert(table, rows, column_names=column_names)
        finally:
            self.putconn(client)


@retry(
    wait=wait_exponential(multiplier=1, min=1, max=10),
    stop=stop_after_delay(30),
    before_sleep=_log_retry
)
def init_clickhouse(pool_size: int = 5) -> ClickHousePool:
    """
    Initializes a pool of ClickHouse clients with an exponential backoff retry mechanism.
    Each client gets its own session, avoiding concurrent-query errors.
    """
    return ClickHousePool(pool_size)

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
        user=os.getenv("POSTGRES_USER", "aer_admin"), # Update with your .env defaults
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