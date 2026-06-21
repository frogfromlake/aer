"""Phase 122e A27 / F-A27 — atomic document-claim compare-and-swap.

Verifies the Postgres-side `try_claim_document` and `release_document_claim`
helpers correctly serialise concurrent claim attempts.

These tests use a real Postgres container via testcontainers because the
correctness property under test (atomic CAS under concurrent updates)
cannot be proven against a mock — the whole point is to exercise
Postgres' MVCC guarantees.
"""

from __future__ import annotations

import threading
import time
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

import pytest
import yaml
from psycopg2.pool import ThreadedConnectionPool
from testcontainers.core.container import DockerContainer

from internal.storage.postgres_client import (
    quarantine_document_status,
    reclaim_stale_processing,
    release_document_claim,
    try_claim_document,
    update_document_status,
)


def _get_compose_image(service_name: str) -> str:
    compose_path = Path(__file__).resolve().parents[3] / "compose.yaml"
    with open(compose_path, "r", encoding="utf-8") as f:
        compose = yaml.safe_load(f)
    return compose["services"][service_name]["image"]


@pytest.fixture(scope="module")
def pg_container():
    container = (
        DockerContainer(_get_compose_image("postgres"))
        .with_env("POSTGRES_DB", "aer_metadata")
        .with_env("POSTGRES_USER", "aer_admin")
        .with_env("POSTGRES_PASSWORD", "aer_secret")
        .with_exposed_ports(5432)
    )
    with container:
        # wait for ready
        import psycopg2
        for _ in range(60):
            try:
                conn = psycopg2.connect(
                    host=container.get_container_host_ip(),
                    port=container.get_exposed_port(5432),
                    user="aer_admin",
                    password="aer_secret",
                    dbname="aer_metadata",
                )
                conn.close()
                break
            except Exception:
                time.sleep(1)
        else:
            raise TimeoutError("Postgres container did not become ready")
        yield container


@pytest.fixture
def pg_pool(pg_container):
    pool = ThreadedConnectionPool(
        minconn=1,
        maxconn=20,
        host=pg_container.get_container_host_ip(),
        port=pg_container.get_exposed_port(5432),
        user="aer_admin",
        password="aer_secret",
        database="aer_metadata",
    )
    # Minimal schema for the claim tests — mirrors infra/postgres/migrations.
    conn = pool.getconn()
    try:
        with conn.cursor() as cur:
            cur.execute("DROP TABLE IF EXISTS documents")
            cur.execute(
                """
                CREATE TABLE documents (
                    id SERIAL PRIMARY KEY,
                    bronze_object_key VARCHAR(500) NOT NULL UNIQUE,
                    status VARCHAR(50),
                    claimed_at TIMESTAMP WITH TIME ZONE
                )
                """
            )
        conn.commit()
    finally:
        pool.putconn(conn)

    yield pool
    pool.closeall()


def _seed_doc(pool, obj_key: str, status: str | None) -> None:
    conn = pool.getconn()
    try:
        with conn.cursor() as cur:
            cur.execute(
                "INSERT INTO documents (bronze_object_key, status) VALUES (%s, %s)",
                (obj_key, status),
            )
        conn.commit()
    finally:
        pool.putconn(conn)


def _read_status(pool, obj_key: str) -> str | None:
    conn = pool.getconn()
    try:
        with conn.cursor() as cur:
            cur.execute(
                "SELECT status FROM documents WHERE bronze_object_key = %s",
                (obj_key,),
            )
            row = cur.fetchone()
            return row[0] if row else None
    finally:
        pool.putconn(conn)


def _read_claimed_at(pool, obj_key: str):
    conn = pool.getconn()
    try:
        with conn.cursor() as cur:
            cur.execute(
                "SELECT claimed_at FROM documents WHERE bronze_object_key = %s",
                (obj_key,),
            )
            row = cur.fetchone()
            return row[0] if row else None
    finally:
        pool.putconn(conn)


def _force_claimed_at(pool, obj_key: str, interval_sql: str) -> None:
    """Set claimed_at to ``now() - <interval_sql>`` (raw SQL, test-only)."""
    conn = pool.getconn()
    try:
        with conn.cursor() as cur:
            cur.execute(
                f"UPDATE documents SET claimed_at = now() - interval '{interval_sql}' "
                "WHERE bronze_object_key = %s",
                (obj_key,),
            )
        conn.commit()
    finally:
        pool.putconn(conn)


def test_claim_succeeds_on_uploaded_status(pg_pool):
    _seed_doc(pg_pool, "k1", "uploaded")
    assert try_claim_document(pg_pool, "k1") is True
    assert _read_status(pg_pool, "k1") == "processing"


def test_claim_succeeds_on_pending_status(pg_pool):
    _seed_doc(pg_pool, "k2", "pending")
    assert try_claim_document(pg_pool, "k2") is True
    assert _read_status(pg_pool, "k2") == "processing"


def test_claim_succeeds_on_null_status(pg_pool):
    _seed_doc(pg_pool, "k3", None)
    assert try_claim_document(pg_pool, "k3") is True
    assert _read_status(pg_pool, "k3") == "processing"


def test_claim_fails_on_already_processed(pg_pool):
    _seed_doc(pg_pool, "k4", "processed")
    assert try_claim_document(pg_pool, "k4") is False
    assert _read_status(pg_pool, "k4") == "processed"


def test_claim_fails_on_quarantined(pg_pool):
    _seed_doc(pg_pool, "k5", "quarantined")
    assert try_claim_document(pg_pool, "k5") is False


def test_claim_fails_on_in_flight_processing(pg_pool):
    """Already-claimed by another worker — second claim returns False."""
    _seed_doc(pg_pool, "k6", "uploaded")
    assert try_claim_document(pg_pool, "k6") is True   # first claim wins
    assert try_claim_document(pg_pool, "k6") is False  # concurrent claim loses


def test_claim_fails_on_nonexistent(pg_pool):
    """Bronze key with no PG row — UPDATE matches zero rows."""
    assert try_claim_document(pg_pool, "nonexistent_key") is False


def test_release_resets_processing_to_uploaded(pg_pool):
    _seed_doc(pg_pool, "k7", "uploaded")
    assert try_claim_document(pg_pool, "k7") is True
    assert release_document_claim(pg_pool, "k7") is True
    assert _read_status(pg_pool, "k7") == "uploaded"
    # After release, a fresh claim wins again.
    assert try_claim_document(pg_pool, "k7") is True


def test_release_no_op_on_processed(pg_pool):
    """Release is CAS-bounded — only releases if status is currently
    'processing'. A 'processed' doc is left alone."""
    _seed_doc(pg_pool, "k8", "processed")
    assert release_document_claim(pg_pool, "k8") is False
    assert _read_status(pg_pool, "k8") == "processed"


def test_release_no_op_on_quarantined(pg_pool):
    _seed_doc(pg_pool, "k9", "quarantined")
    assert release_document_claim(pg_pool, "k9") is False
    assert _read_status(pg_pool, "k9") == "quarantined"


def test_concurrent_claims_serialise_exactly_one_winner(pg_pool):
    """The race that F-A27 closes: N concurrent threads attempt to claim
    the same doc; exactly one wins."""
    _seed_doc(pg_pool, "k_race", "uploaded")

    n_threads = 16
    barrier = threading.Barrier(n_threads)
    wins = []
    lock = threading.Lock()

    def attempt():
        barrier.wait()  # synchronise the race start
        won = try_claim_document(pg_pool, "k_race")
        with lock:
            wins.append(won)

    with ThreadPoolExecutor(max_workers=n_threads) as ex:
        list(ex.map(lambda _: attempt(), range(n_threads)))

    assert sum(1 for w in wins if w) == 1, f"expected exactly 1 winner, got {sum(wins)}"
    assert _read_status(pg_pool, "k_race") == "processing"


def test_terminal_status_after_claim_is_preserved(pg_pool):
    """Worker claims, then terminally updates status — release is a no-op."""
    _seed_doc(pg_pool, "k_term", "uploaded")
    assert try_claim_document(pg_pool, "k_term") is True
    update_document_status(pg_pool, "k_term", "processed")
    # Late release attempt (e.g., from a finally-block in a happy-path
    # worker) must not regress 'processed' to 'uploaded'.
    assert release_document_claim(pg_pool, "k_term") is False
    assert _read_status(pg_pool, "k_term") == "processed"


# --- SEC-074: claimed_at + stale-processing reaper ------------------------

def test_claim_stamps_claimed_at(pg_pool):
    """A winning claim records claimed_at so the reaper can age it (SEC-074)."""
    _seed_doc(pg_pool, "c1", "uploaded")
    assert _read_claimed_at(pg_pool, "c1") is None
    assert try_claim_document(pg_pool, "c1") is True
    assert _read_claimed_at(pg_pool, "c1") is not None


def test_reaper_reclaims_stale_processing(pg_pool):
    """A claim older than the threshold is reset to 'uploaded' and returned."""
    _seed_doc(pg_pool, "stale1", "uploaded")
    assert try_claim_document(pg_pool, "stale1") is True
    _force_claimed_at(pg_pool, "stale1", "1 hour")
    reclaimed = reclaim_stale_processing(pg_pool, threshold_seconds=900)
    assert "stale1" in reclaimed
    assert _read_status(pg_pool, "stale1") == "uploaded"
    assert _read_claimed_at(pg_pool, "stale1") is None


def test_reaper_leaves_fresh_claims_alone(pg_pool):
    """A merely-slow live worker (fresh claim) is never robbed of its claim."""
    _seed_doc(pg_pool, "fresh1", "uploaded")
    assert try_claim_document(pg_pool, "fresh1") is True
    reclaimed = reclaim_stale_processing(pg_pool, threshold_seconds=900)
    assert "fresh1" not in reclaimed
    assert _read_status(pg_pool, "fresh1") == "processing"


def test_reaper_reclaims_null_claimed_at_strays(pg_pool):
    """A pre-migration 'processing' row (claimed_at NULL) is reclaimable."""
    _seed_doc(pg_pool, "stray1", "processing")  # no claim stamp
    reclaimed = reclaim_stale_processing(pg_pool, threshold_seconds=900)
    assert "stray1" in reclaimed
    assert _read_status(pg_pool, "stray1") == "uploaded"


def test_reaper_ignores_terminal_rows(pg_pool):
    """Reaper only touches 'processing' — never processed/quarantined."""
    _seed_doc(pg_pool, "done1", "processed")
    _seed_doc(pg_pool, "dlq1", "quarantined")
    reclaimed = reclaim_stale_processing(pg_pool, threshold_seconds=0)
    assert "done1" not in reclaimed and "dlq1" not in reclaimed
    assert _read_status(pg_pool, "done1") == "processed"
    assert _read_status(pg_pool, "dlq1") == "quarantined"


# --- SEC-065: CAS-guarded poison quarantine write -------------------------

def test_quarantine_status_cas_skips_processed(pg_pool):
    """The poison status write must never clobber a terminal 'processed'."""
    _seed_doc(pg_pool, "p1", "processed")
    assert quarantine_document_status(pg_pool, "p1") is False
    assert _read_status(pg_pool, "p1") == "processed"


def test_quarantine_status_cas_marks_non_terminal(pg_pool):
    """A non-'processed' row is transitioned to 'quarantined'."""
    _seed_doc(pg_pool, "p2", "processing")
    assert quarantine_document_status(pg_pool, "p2") is True
    assert _read_status(pg_pool, "p2") == "quarantined"
