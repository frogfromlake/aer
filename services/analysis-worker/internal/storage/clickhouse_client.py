import os
import queue
import structlog
import clickhouse_connect
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
