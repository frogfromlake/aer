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

import time
import urllib.request
import urllib.error
import pytest
import psycopg2
import yaml
from pathlib import Path
from unittest.mock import patch, MagicMock

from tenacity import RetryError
from testcontainers.postgres import PostgresContainer
from testcontainers.core.container import DockerContainer

from internal.storage import init_postgres, init_minio, init_clickhouse

# ---------------------------------------------------------------------------
# SSoT: Dynamic Compose Parsing
# ---------------------------------------------------------------------------

def get_compose_image(service_name: str) -> str:
    """Parses the compose.yaml at the repo root to find the image for a service."""
    compose_path = Path(__file__).resolve().parents[3] / "compose.yaml"

    with open(compose_path, "r", encoding="utf-8") as f:
        compose = yaml.safe_load(f)

    try:
        return compose["services"][service_name]["image"]
    except KeyError:
        raise ValueError(f"Image for service '{service_name}' not found in compose.yaml")

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
        image=get_compose_image("postgres"),
        username="aer_admin",
        password="aer_secret",
        dbname="aer_metadata",
    ) as container:
        yield container


@pytest.fixture(scope="module")
def minio_container():
    """Starts an isolated MinIO container for the duration of this module."""
    container = (
        DockerContainer(get_compose_image("minio"))
        .with_env("MINIO_ROOT_USER", "minioadmin")
        .with_env("MINIO_ROOT_PASSWORD", "minioadmin")
        .with_command("server /data")
        .with_exposed_ports(9000)
    )
    with container:
        # Robust HTTP wait strategy instead of brittle log parsing
        start_time = time.time()
        is_ready = False
        
        while time.time() - start_time < 30:
            try:
                host = container.get_container_host_ip()
                port = container.get_exposed_port(9000)
                # MinIO native healthcheck endpoint
                response = urllib.request.urlopen(f"http://{host}:{port}/minio/health/live", timeout=1)
                
                if response.getcode() == 200:
                    is_ready = True
                    break
            except (urllib.error.URLError, ConnectionError, TimeoutError):
                time.sleep(1)
                
        if not is_ready:
            raise TimeoutError("MinIO container did not become ready within 30 seconds.")
            
        yield container


@pytest.fixture(scope="module")
def ch_container():
    """Starts an isolated ClickHouse container for the duration of this module."""
    container = (
        DockerContainer(get_compose_image("clickhouse"))
        .with_env("CLICKHOUSE_USER", "aer_admin")
        .with_env("CLICKHOUSE_PASSWORD", "aer_secret")
        .with_env("CLICKHOUSE_DB", "aer_gold")
        .with_exposed_ports(8123)
    )
    with container:
        # Robust HTTP wait strategy instead of brittle log parsing
        start_time = time.time()
        is_ready = False
        
        while time.time() - start_time < 60:
            try:
                host = container.get_container_host_ip()
                port = container.get_exposed_port(8123)
                response = urllib.request.urlopen(f"http://{host}:{port}/ping", timeout=1)
                
                if response.getcode() == 200:
                    is_ready = True
                    break
            except (urllib.error.URLError, ConnectionError, TimeoutError):
                time.sleep(1)
                
        if not is_ready:
            raise TimeoutError("ClickHouse container did not become ready within 60 seconds.")
            
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
        monkeypatch.setenv("POSTGRES_PORT", str(pg_container.get_exposed_port(5432)))
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
        Verifies that init_clickhouse() returns a ClickHousePool against
        a real ClickHouse instance and can execute inserts.
        """
        monkeypatch.setenv("CLICKHOUSE_HOST", ch_container.get_container_host_ip())
        monkeypatch.setenv("CLICKHOUSE_PORT", str(ch_container.get_exposed_port(8123)))
        monkeypatch.setenv("CLICKHOUSE_USER", "aer_admin")
        monkeypatch.setenv("CLICKHOUSE_PASSWORD", "aer_secret")
        monkeypatch.setenv("CLICKHOUSE_DB", "aer_gold")

        pool = init_clickhouse(pool_size=2)
        assert pool is not None

        # Verify connections are functional via getconn/putconn
        client = pool.getconn()
        try:
            result = client.query("SELECT 1")
            assert result.result_rows == [(1,)]
        finally:
            pool.putconn(client)

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
            pool = init_clickhouse(pool_size=1)

        assert pool is not None
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
                init_clickhouse(pool_size=1)
