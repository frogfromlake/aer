"""
Tests for boot-time credential validation (Phase 90).

The worker must refuse to start when required credentials are empty or
unset, mirroring the Go services' fail-fast pattern.
"""

import pytest

from main import validate_required_env


class TestValidateRequiredEnv:
    """validate_required_env raises SystemExit when credentials are missing."""

    def test_raises_when_var_is_unset(self, monkeypatch):
        monkeypatch.delenv("POSTGRES_PASSWORD", raising=False)
        with pytest.raises(SystemExit, match="POSTGRES_PASSWORD"):
            validate_required_env(["POSTGRES_PASSWORD"])

    def test_raises_when_var_is_empty(self, monkeypatch):
        monkeypatch.setenv("POSTGRES_PASSWORD", "")
        with pytest.raises(SystemExit, match="POSTGRES_PASSWORD"):
            validate_required_env(["POSTGRES_PASSWORD"])

    def test_raises_when_var_is_whitespace_only(self, monkeypatch):
        monkeypatch.setenv("MINIO_ACCESS_KEY", "   ")
        with pytest.raises(SystemExit, match="MINIO_ACCESS_KEY"):
            validate_required_env(["MINIO_ACCESS_KEY"])

    def test_reports_all_missing_vars(self, monkeypatch):
        monkeypatch.delenv("POSTGRES_PASSWORD", raising=False)
        monkeypatch.delenv("CLICKHOUSE_PASSWORD", raising=False)
        with pytest.raises(SystemExit, match="POSTGRES_PASSWORD") as exc_info:
            validate_required_env(["POSTGRES_PASSWORD", "CLICKHOUSE_PASSWORD"])
        assert "CLICKHOUSE_PASSWORD" in str(exc_info.value)

    def test_passes_when_all_vars_set(self, monkeypatch):
        monkeypatch.setenv("POSTGRES_PASSWORD", "secret1")
        monkeypatch.setenv("MINIO_ACCESS_KEY", "key")
        monkeypatch.setenv("MINIO_SECRET_KEY", "secret2")
        monkeypatch.setenv("CLICKHOUSE_PASSWORD", "secret3")
        # Should not raise
        validate_required_env([
            "POSTGRES_PASSWORD",
            "MINIO_ACCESS_KEY",
            "MINIO_SECRET_KEY",
            "CLICKHOUSE_PASSWORD",
        ])

    def test_partial_missing_reports_only_missing(self, monkeypatch):
        monkeypatch.setenv("POSTGRES_PASSWORD", "ok")
        monkeypatch.delenv("MINIO_SECRET_KEY", raising=False)
        with pytest.raises(SystemExit, match="MINIO_SECRET_KEY") as exc_info:
            validate_required_env(["POSTGRES_PASSWORD", "MINIO_SECRET_KEY"])
        assert "POSTGRES_PASSWORD" not in str(exc_info.value)
