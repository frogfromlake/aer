"""WikidataAliasIndex per-thread sqlite connections.

The worker resolves entities from an executor thread-pool; a single shared
sqlite3 connection raises SQLITE_MISUSE ("bad parameter or other API misuse")
under concurrency. The index must hand each thread its own read-only
connection, track them all, and close them on shutdown.
"""

from __future__ import annotations

import sqlite3
import threading
from pathlib import Path

from internal.storage.wikidata_alias_index import WikidataAliasIndex


def _build_index(tmp_path: Path) -> WikidataAliasIndex:
    db_path = tmp_path / "aliases.db"
    conn = sqlite3.connect(str(db_path))
    conn.execute("CREATE TABLE aliases (alias TEXT PRIMARY KEY, qid TEXT, label TEXT)")
    conn.execute("INSERT INTO aliases VALUES ('merkel', 'Q567', 'Angela Merkel')")
    conn.commit()
    conn.close()
    return WikidataAliasIndex(str(db_path))


def test_per_thread_connections_are_distinct(tmp_path: Path) -> None:
    """Each thread must get its OWN sqlite connection (the anti-MISUSE guarantee)."""
    index = _build_index(tmp_path)
    assert index.resolve("merkel") == ("Q567", "Angela Merkel")

    ids: dict[str, int] = {}
    lock = threading.Lock()

    def worker(name: str) -> None:
        conn_id = id(index._conn())
        index.resolve("merkel")  # exercise a real query on this thread's conn
        with lock:
            ids[name] = conn_id

    threads = [threading.Thread(target=worker, args=(f"t{i}",)) for i in range(4)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()

    # 4 worker threads → 4 distinct connection objects.
    assert len(set(ids.values())) == 4
    # 4 workers + the constructing (test) thread = 5 tracked connections.
    assert len(index._all_conns) == 5

    index.close()
    assert index._all_conns == []
