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
        self._pool_timeout = int(os.getenv("CLICKHOUSE_POOL_TIMEOUT_SECONDS", "30"))
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
        try:
            return self._pool.get(timeout=self._pool_timeout)
        except queue.Empty:
            raise TimeoutError(
                f"ClickHouse pool exhausted: no client available within {self._pool_timeout}s"
            )

    def putconn(self, client: clickhouse_connect.driver.Client) -> None:
        self._pool.put(client)

    def close_all(self) -> None:
        """Drain the pool and close every ClickHouse client connection."""
        while not self._pool.empty():
            try:
                client = self._pool.get_nowait()
                client.close()
            except queue.Empty:
                break

    def insert(self, table: str, rows: list, column_names: list[str], deduplication_token: str | None = None) -> None:
        """Insert rows into a ClickHouse table.

        When `deduplication_token` is provided, it is forwarded as the
        ClickHouse `insert_deduplication_token` setting. Combined with
        `non_replicated_deduplication_window` on the target table
        (set via migration 000021), ClickHouse refuses any subsequent
        INSERT block carrying the same token — silently no-oping
        retries from at-least-once message delivery before the
        source-table dedup check fires. See Phase 122e A19 / F-A19.

        Phase 122e A27 / F-A27 — note on MV dedup. The ClickHouse setting
        ``deduplicate_blocks_in_dependent_materialized_views=1`` would
        propagate source-side dedup to dependent MVs, BUT it only takes
        effect when the source table is a ``Replicated*MergeTree``. Our
        schema uses non-Replicated ``ReplacingMergeTree``, so MV dedup
        cannot be enforced at this layer. The race condition that
        produces raw-vs-MV drift (NATS redelivery + non-atomic worker
        idempotency check) is therefore eliminated upstream — see the
        atomic-claim implementation in
        ``internal/storage/postgres_client.try_claim_document``. Once a
        single worker holds the claim per ``bronze_object_key``, no
        duplicate insert ever reaches ClickHouse, and the MV stays
        aligned by construction.
        """
        client = self.getconn()
        try:
            if deduplication_token is not None:
                client.insert(
                    table,
                    rows,
                    column_names=column_names,
                    settings={"insert_deduplication_token": deduplication_token},
                )
            else:
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
