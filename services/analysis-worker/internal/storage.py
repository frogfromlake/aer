import os
import structlog
import clickhouse_connect
from minio import Minio
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

@retry(
    wait=wait_exponential(multiplier=1, min=1, max=10),
    stop=stop_after_delay(30),
    before_sleep=_log_retry
)
def init_clickhouse() -> clickhouse_connect.driver.Client:
    """
    Initializes the ClickHouse client with an exponential backoff retry mechanism.
    The get_client method automatically pings the server.
    """
    client = clickhouse_connect.get_client(
        host=os.getenv("CLICKHOUSE_HOST", "localhost"),
        port=int(os.getenv("CLICKHOUSE_PORT", "8123")),
        username=os.getenv("CLICKHOUSE_USER", "default"),
        password=os.getenv("CLICKHOUSE_PASSWORD", ""),
        database=os.getenv("CLICKHOUSE_DB", "aer_gold")
    )
    return client