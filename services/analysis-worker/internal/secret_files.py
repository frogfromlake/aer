"""The ``<KEY>_FILE`` convention for the analysis worker (Phase 155 / ADR-046).

A credential may be supplied either directly as the environment variable
``<KEY>`` or as a path in ``<KEY>_FILE`` pointing to a file whose contents are
the value. The file form is how Docker secrets deliver credentials: Compose
mounts each secret as a tmpfs file under ``/run/secrets/*`` and passes
``<KEY>_FILE`` pointing at it, so the value never lands in the container's
on-disk config. The ``_FILE`` form takes precedence; absent it, behaviour is
unchanged (the plain env / ``.env`` value is used), so it is backward-compatible.
"""

from __future__ import annotations

import os


def load_file_secrets(names: list[str]) -> None:
    """Resolve the ``<name>_FILE`` convention into ``os.environ`` for each name.

    For every name whose ``<name>_FILE`` env var points to a readable file, the
    file's contents (a single trailing newline stripped) replace
    ``os.environ[name]``. Names with no ``<name>_FILE`` set are left untouched.

    A configured-but-unreadable ``<name>_FILE`` raises ``SystemExit`` — the
    worker must fail fast rather than boot with a missing credential, mirroring
    :func:`main.validate_required_env`. Must run before any code reads the
    secret env vars (i.e. before ``validate_required_env`` and pool creation).
    """
    for name in names:
        path = os.getenv(f"{name}_FILE", "").strip()
        if not path:
            continue
        try:
            with open(path, encoding="utf-8") as handle:
                # Strip only a trailing newline (the common file-write artefact);
                # leave other whitespace intact in case a secret ends in a space.
                os.environ[name] = handle.read().rstrip("\r\n")
        except OSError as exc:
            raise SystemExit(
                f"Fatal: {name}_FILE is set ({path}) but unreadable: {exc}"
            ) from exc
