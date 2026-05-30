"""Wikidata alias index — Phase 118 (ADR-016 Tier-1.5 NER alias resolution).

A read-only SQLite sidecar mapping lowercased entity aliases to canonical
Wikidata QIDs + English labels. Built offline (see
`scripts/build/build_wikidata_alias_index.py`) and shipped as a baked
image artefact; the worker opens it read-only and never writes.

Phase 118: exact-match lookup only. The alias table is pre-lowercased at
build time so the lookup is a single indexed equality probe.

Thread-safety (Phase 123): the worker resolves entities from an asyncio
executor thread-pool, so this index is touched concurrently by many threads.
A single ``sqlite3.Connection`` is NOT safe for concurrent use — even with
``check_same_thread=False``, which only disables Python's ownership *check*.
Concurrent ``execute()`` calls on one connection interleave on its shared
statement state and raise ``sqlite3.ProgrammingError: bad parameter or other
API misuse`` (SQLITE_MISUSE) — observed under high-throughput crawls.
Read-only SQLite instead supports unlimited *independent* connections to the
same file, so we keep one connection (and one hot-alias cache) PER THREAD via
``threading.local``: no shared mutable state on the hot path, no locks, no
MISUSE.
"""

from __future__ import annotations

import sqlite3
import threading
import structlog
from pathlib import Path
from typing import Optional

logger = structlog.get_logger()


class WikidataAliasIndex:
    """Read-only alias→(QID, label) lookup over a baked SQLite file.

    Each worker thread lazily opens its own read-only connection and keeps its
    own bounded hot-alias cache (see module docstring for the thread-safety
    rationale).
    """

    def __init__(self, db_path: str, *, expected_sha256: Optional[str] = None) -> None:
        self._db_path = db_path
        path = Path(db_path)
        if not path.is_file():
            raise FileNotFoundError(f"Wikidata alias index not found: {db_path}")

        if expected_sha256:
            self._verify_integrity(path, expected_sha256)

        # Phase 118: in-process LRU over the hot alias set. The worker sees the
        # same handful of entities (politicians, ministries) on nearly every
        # article, so a small per-thread cache turns the SQLite probe into a
        # dict hit for the common case.
        self._cache_capacity = 4096

        # Per-thread connection + cache. `threading.local` gives each executor
        # thread its own connection; `_all_conns` tracks them so close() can
        # release every one at shutdown (the list is touched only on
        # first-use-per-thread and on close, so the lock is uncontended).
        self._local = threading.local()
        self._all_conns: list[sqlite3.Connection] = []
        self._conns_lock = threading.Lock()

        # Open one connection on the constructing thread now — fail fast on a
        # missing/corrupt file and validate the schema once at startup.
        self._verify_schema(self._conn())
        logger.info("Wikidata alias index opened", path=db_path)

    def _conn(self) -> sqlite3.Connection:
        """Return this thread's read-only connection, opening it on first use."""
        conn: Optional[sqlite3.Connection] = getattr(self._local, "conn", None)
        if conn is None:
            # mode=ro guarantees no writes and fails fast if the file vanished.
            # check_same_thread=False lets close() run from the shutdown thread;
            # each connection is otherwise used only by its owning thread.
            conn = sqlite3.connect(
                f"file:{self._db_path}?mode=ro",
                uri=True,
                check_same_thread=False,
            )
            conn.row_factory = sqlite3.Row
            self._local.conn = conn
            self._local.cache = {}
            with self._conns_lock:
                self._all_conns.append(conn)
        return conn

    def _verify_integrity(self, path: Path, expected_sha256: str) -> None:
        import hashlib

        digest = hashlib.sha256(path.read_bytes()).hexdigest()
        if digest != expected_sha256:
            raise ValueError(
                f"Wikidata alias index sha256 mismatch: expected {expected_sha256}, got {digest}"
            )

    def _verify_schema(self, conn: sqlite3.Connection) -> None:
        cur = conn.execute(
            "SELECT name FROM sqlite_master WHERE type='table' AND name='aliases'"
        )
        if cur.fetchone() is None:
            raise ValueError("Wikidata alias index missing 'aliases' table")

    def resolve(self, alias: str) -> Optional[tuple[str, str]]:
        """Return (qid, label) for an exact lowercased alias match, or None."""
        if not alias:
            return None
        conn = self._conn()
        cache: dict[str, Optional[tuple[str, str]]] = self._local.cache
        key = alias.lower()
        if key in cache:
            return cache[key]
        cur = conn.execute(
            "SELECT qid, label FROM aliases WHERE alias = ? LIMIT 1",
            (key,),
        )
        row = cur.fetchone()
        result: Optional[tuple[str, str]] = (row["qid"], row["label"]) if row else None
        cache[key] = result
        if len(cache) > self._cache_capacity:
            cache.clear()
        return result

    def close(self) -> None:
        """Close every per-thread connection (best-effort; safe to call once)."""
        with self._conns_lock:
            for conn in self._all_conns:
                try:
                    conn.close()
                except Exception:
                    pass
            self._all_conns.clear()
