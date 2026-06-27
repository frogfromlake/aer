"""Tests for the <KEY>_FILE convention (Phase 155 / ADR-046)."""

from __future__ import annotations

import os

import pytest

from internal.secret_files import load_file_secrets


def test_reads_value_from_file_and_overrides_env(tmp_path, monkeypatch):
    secret = tmp_path / "pw"
    secret.write_text("file-secret\n", encoding="utf-8")
    monkeypatch.setenv("POSTGRES_PASSWORD", "env-secret")
    monkeypatch.setenv("POSTGRES_PASSWORD_FILE", str(secret))

    load_file_secrets(["POSTGRES_PASSWORD"])

    assert os.environ["POSTGRES_PASSWORD"] == "file-secret"


def test_strips_only_trailing_newline(tmp_path, monkeypatch):
    secret = tmp_path / "pw"
    secret.write_text("p4ss  \r\n", encoding="utf-8")  # trailing spaces preserved
    monkeypatch.setenv("CLICKHOUSE_PASSWORD_FILE", str(secret))

    load_file_secrets(["CLICKHOUSE_PASSWORD"])

    assert os.environ["CLICKHOUSE_PASSWORD"] == "p4ss  "


def test_no_file_env_leaves_value_untouched(monkeypatch):
    monkeypatch.setenv("WORKER_MINIO_ACCESS_KEY", "plain-env")
    monkeypatch.delenv("WORKER_MINIO_ACCESS_KEY_FILE", raising=False)

    load_file_secrets(["WORKER_MINIO_ACCESS_KEY"])

    assert os.environ["WORKER_MINIO_ACCESS_KEY"] == "plain-env"


def test_blank_file_env_is_ignored(monkeypatch):
    monkeypatch.setenv("WORKER_MINIO_SECRET_KEY", "v")
    monkeypatch.setenv("WORKER_MINIO_SECRET_KEY_FILE", "   ")

    load_file_secrets(["WORKER_MINIO_SECRET_KEY"])

    assert os.environ["WORKER_MINIO_SECRET_KEY"] == "v"


def test_unreadable_file_is_fatal(tmp_path, monkeypatch):
    monkeypatch.setenv("POSTGRES_PASSWORD_FILE", str(tmp_path / "missing"))

    with pytest.raises(SystemExit):
        load_file_secrets(["POSTGRES_PASSWORD"])
