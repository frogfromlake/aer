"""
Storage package for AĒR Analysis Worker.

Re-exports all public symbols so that external import paths remain unchanged.
`Minio` and `clickhouse_connect` are imported here for test patch compatibility —
patching `internal.storage.Minio.list_buckets` and
`internal.storage.clickhouse_connect.get_client` targets the actual class/module
objects, which are singletons shared across all submodules.
"""
from minio import Minio  # noqa: F401 — kept for test patch target compatibility
import clickhouse_connect  # noqa: F401 — kept for test patch target compatibility

from internal.storage.minio_client import init_minio
from internal.storage.clickhouse_client import init_clickhouse, ClickHousePool
from internal.storage.postgres_client import init_postgres

__all__ = ["init_minio", "init_clickhouse", "init_postgres", "ClickHousePool"]
