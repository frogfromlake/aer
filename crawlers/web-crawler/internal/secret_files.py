"""The ``<KEY>_FILE`` convention for the web crawler (Phase 155 / ADR-046).

A credential may be supplied either directly as the environment variable
``<KEY>`` or as a path in ``<KEY>_FILE`` pointing to a file whose contents are
the value. The file form is how Docker secrets deliver credentials (tmpfs
``/run/secrets/*``) without the value touching the container's on-disk config.
The ``_FILE`` form takes precedence; absent it, behaviour is unchanged, so it is
backward-compatible. Mirrors ``services/analysis-worker/internal/secret_files``.
"""

from __future__ import annotations

import os


def load_file_secrets(names: list[str]) -> None:
    """Resolve the ``<name>_FILE`` convention into ``os.environ`` for each name.

    For every name whose ``<name>_FILE`` env var points to a readable file, the
    file's contents (a single trailing newline stripped) replace
    ``os.environ[name]``. Names with no ``<name>_FILE`` set are untouched. A
    configured-but-unreadable ``<name>_FILE`` raises ``SystemExit`` (fail fast).
    """
    for name in names:
        path = os.getenv(f"{name}_FILE", "").strip()
        if not path:
            continue
        try:
            with open(path, encoding="utf-8") as handle:
                os.environ[name] = handle.read().rstrip("\r\n")
        except OSError as exc:
            raise SystemExit(
                f"Fatal: {name}_FILE is set ({path}) but unreadable: {exc}"
            ) from exc
