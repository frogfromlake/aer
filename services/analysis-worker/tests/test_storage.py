"""
Integration tests for services/analysis-worker/internal/storage.py

These tests validate two concerns:

1. **Connection tests** (via testcontainers): Each `init_*` function must
   successfully connect to a real, isolated infrastructure container and
   return a usable client object.

2. **Retry behavior tests** (via mocks + patched sleep): Each `init_*`
   function must retry when the underlying service is temporarily unavailable,
   and eventually raise `tenacity.RetryError` when the stop condition is reached.

Requirements: Docker must be available. Run via `make test-python`.
"""

import pytest
import psycopg2
from unittest.mock import patch, MagicMock

from tenacity import RetryError
from testcontainers.postgres import PostgresContainer
from testcontainers.core.container import DockerContainer
from testcontainers.core.waiting_utils import wait_for_logs

from internal.storage import init_postgres, init_minio, init_clickhouse


# ---------------------------------------------------------------------------
# Shared helpers
# ---------------------------------------------------------------------------

def _make_cursor_mock():
    """Returns a mock that behaves like a psycopg2 cursor context manager."""
    cur = MagicMock()
    cur.__enter__ = lambda s: s
    cur.__exit__ = MagicMock(return_value=False)
    return cur


# ---------------------------------------------------------------------------
# Container fixtures (module-scoped to amortize startup cost across tests)
# ---------------------------------------------------------------------------

@pytest.fixture(scope="module")
def pg_container():
    """Starts an isolated PostgreSQL container for the duration of this module."""
    with PostgresContainer(
        image="postgres:15-alpine",
        username="aer_admin",
        password="aer_secret",
        dbname="aer_metadata",
    ) as container:
        yield container


@pytest.fixture(scope="module")
def minio_container():
    """Starts an isolated MinIO container for the duration of this module."""
    container = (
        DockerContainer("minio/minio:latest")
        .with_env("MINIO_ROOT_USER", "minioadmin")
        .with_env("MINIO_ROOT_PASSWORD", "minioadmin")
        .with_command("server /data")
        .with_exposed_ports(9000)
    )
    with container:
        wait_for_logs(container, "API", timeout=30)
        yield container


@pytest.fixture(scope="module")
def ch_container():
    """Starts an isolated ClickHouse container for the duration of this module."""
    container = (
        DockerContainer("clickhouse/clickhouse-server:24-alpine")
        .with_exposed_ports(8123)
    )
    with container:
        wait_for_logs(container, "Ready for connections", timeout=60)
        yield container


# ---------------------------------------------------------------------------
# PostgreSQL — connection & retry
# ---------------------------------------------------------------------------

class TestInitPostgres:
    def test_connects_successfully(self, pg_container, monkeypatch):
        """
        Verifies that init_postgres() returns a functional ThreadedConnectionPool
        when pointed at a real PostgreSQL instance.
        """
        monkeypatch.setenv("POSTGRES_HOST", pg_container.get_container_host_ip())
        monkeypatch.setenv("POSTGRES_PORT", pg_container.get_exposed_port(5432))
        monkeypatch.setenv("POSTGRES_USER", "aer_admin")
        monkeypatch.setenv("POSTGRES_PASSWORD", "aer_secret")
        monkeypatch.setenv("POSTGRES_DB", "aer_metadata")

        pool = init_postgres()
        assert pool is not None

        conn = pool.getconn()
        try:
            with conn.cursor() as cur:
                cur.execute("SELECT 1")
                assert cur.fetchone() == (1,)
        finally:
            pool.putconn(conn)
            pool.closeall()

    def test_retries_on_transient_failure(self, monkeypatch):
        """
        Verifies that init_postgres() retries when the DB is temporarily unavailable.
        The pool constructor fails twice, then succeeds on the third attempt.
        Sleep is patched to keep the test fast without weakening the retry logic.
        """
        attempt_count = 0

        def flaky_pool_factory(*args, **kwargs):
            nonlocal attempt_count
            attempt_count += 1
            if attempt_count < 3:
                raise psycopg2.OperationalError("Connection refused (simulated)")
            mock_pool = MagicMock()
            mock_conn = MagicMock()
            mock_pool.getconn.return_value = mock_conn
            mock_conn.cursor.return_value = _make_cursor_mock()
            return mock_pool

        with patch("internal.storage.ThreadedConnectionPool", side_effect=flaky_pool_factory), \
             patch("time.sleep"):
            pool = init_postgres()

        assert pool is not None
        assert attempt_count == 3

    def test_raises_after_stop_delay_exceeded(self, monkeypatch):
        """
        Verifies that init_postgres() raises tenacity.RetryError after all retries
        are exhausted (stop_after_delay reached).
        """
        monkeypatch.setenv("POSTGRES_HOST", "127.0.0.1")
        monkeypatch.setenv("POSTGRES_PORT", "19999")  # Unreachable port

        with patch("internal.storage.ThreadedConnectionPool",
                   side_effect=psycopg2.OperationalError("always failing")), \
             patch("time.sleep"), \
             patch("time.monotonic", side_effect=[0.0, 0.5, 1.0, 31.0]):
            with pytest.raises(RetryError):
                init_postgres()


# ---------------------------------------------------------------------------
# MinIO — connection & retry
# ---------------------------------------------------------------------------

class TestInitMinio:
    def test_connects_successfully(self, minio_container, monkeypatch):
        """
        Verifies that init_minio() returns a Minio client that can list buckets
        against a real MinIO instance.
        """
        monkeypatch.setenv("MINIO_ENDPOINT",
                           f"{minio_container.get_container_host_ip()}:"
                           f"{minio_container.get_exposed_port(9000)}")
        monkeypatch.setenv("MINIO_ROOT_USER", "minioadmin")
        monkeypatch.setenv("MINIO_ROOT_PASSWORD", "minioadmin")

        client = init_minio()
        assert client is not None
        # list_buckets() must not raise — proves the connection is live
        buckets = client.list_buckets()
        assert isinstance(buckets, list)

    def test_retries_on_transient_failure(self, monkeypatch):
        """
        Verifies that init_minio() retries when list_buckets() fails transiently.
        The probe call fails twice, then succeeds on the third attempt.
        """
        attempt_count = 0

        def flaky_list_buckets(self_):  # noqa: N803
            nonlocal attempt_count
            attempt_count += 1
            if attempt_count < 3:
                raise Exception("MinIO not ready (simulated)")
            return []

        with patch("internal.storage.Minio.list_buckets", flaky_list_buckets), \
             patch("time.sleep"):
            client = init_minio()

        assert client is not None
        assert attempt_count == 3

    def test_raises_after_stop_delay_exceeded(self):
        """
        Verifies that init_minio() raises tenacity.RetryError after all retries
        are exhausted.
        """
        with patch("internal.storage.Minio.list_buckets",
                   side_effect=Exception("always unavailable")), \
             patch("time.sleep"), \
             patch("time.monotonic", side_effect=[0.0, 0.5, 1.0, 31.0]):
            with pytest.raises(RetryError):
                init_minio()


# ---------------------------------------------------------------------------
# ClickHouse — connection & retry
# ---------------------------------------------------------------------------

class TestInitClickhouse:
    def test_connects_successfully(self, ch_container, monkeypatch):
        """
        Verifies that init_clickhouse() returns a live client against
        a real ClickHouse instance and can execute a simple query.
        """
        monkeypatch.setenv("CLICKHOUSE_HOST", ch_container.get_container_host_ip())
        monkeypatch.setenv("CLICKHOUSE_PORT", ch_container.get_exposed_port(8123))
        monkeypatch.setenv("CLICKHOUSE_USER", "default")
        monkeypatch.setenv("CLICKHOUSE_PASSWORD", "")
        monkeypatch.setenv("CLICKHOUSE_DB", "default")

        client = init_clickhouse()
        assert client is not None

        # Verify the connection is functional
        result = client.query("SELECT 1")
        assert result.result_rows == [(1,)]

    def test_retries_on_transient_failure(self):
        """
        Verifies that init_clickhouse() retries when get_client fails transiently.
        The factory raises twice, then returns a mock client on the third attempt.
        """
        attempt_count = 0

        def flaky_get_client(**kwargs):
            nonlocal attempt_count
            attempt_count += 1
            if attempt_count < 3:
                raise Exception("ClickHouse not ready (simulated)")
            return MagicMock()

        with patch("internal.storage.clickhouse_connect.get_client",
                   side_effect=flaky_get_client), \
             patch("time.sleep"):
            client = init_clickhouse()

        assert client is not None
        assert attempt_count == 3

    def test_raises_after_stop_delay_exceeded(self):
        """
        Verifies that init_clickhouse() raises tenacity.RetryError after all
        retries are exhausted.
        """
        with patch("internal.storage.clickhouse_connect.get_client",
                   side_effect=Exception("always unavailable")), \
             patch("time.sleep"), \
             patch("time.monotonic", side_effect=[0.0, 0.5, 1.0, 31.0]):
            with pytest.raises(RetryError):
                init_clickhouse()
