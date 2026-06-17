"""Unit tests for the worker's testable main.py helpers.

The NATS-lifecycle entrypoint `main()` itself is integration territory (excluded
per ADR-041); these cover the pure/boot-time helpers around it.
"""

from __future__ import annotations

from types import SimpleNamespace

import pytest

from main import _num_delivered, validate_required_env


# --- validate_required_env (boot-time secret validation) ---------------------


def test_validate_required_env_passes_when_all_set(monkeypatch):
    monkeypatch.setenv("AER_TEST_REQUIRED", "value")
    validate_required_env(["AER_TEST_REQUIRED"])  # must not raise


def test_validate_required_env_raises_when_missing(monkeypatch):
    monkeypatch.delenv("AER_TEST_ABSENT", raising=False)
    with pytest.raises(SystemExit) as exc:
        validate_required_env(["AER_TEST_ABSENT"])
    assert "AER_TEST_ABSENT" in str(exc.value)


def test_validate_required_env_treats_whitespace_as_empty(monkeypatch):
    monkeypatch.setenv("AER_TEST_BLANK", "   ")
    with pytest.raises(SystemExit):
        validate_required_env(["AER_TEST_BLANK"])


def test_validate_required_env_reports_all_missing(monkeypatch):
    monkeypatch.delenv("AER_TEST_A", raising=False)
    monkeypatch.delenv("AER_TEST_B", raising=False)
    with pytest.raises(SystemExit) as exc:
        validate_required_env(["AER_TEST_A", "AER_TEST_B"])
    message = str(exc.value)
    assert "AER_TEST_A" in message and "AER_TEST_B" in message


# --- _num_delivered (JetStream delivery count, fail-safe) --------------------


def test_num_delivered_returns_count():
    msg = SimpleNamespace(metadata=SimpleNamespace(num_delivered=3))
    assert _num_delivered(msg) == 3


def test_num_delivered_zero_when_metadata_unavailable():
    assert _num_delivered(SimpleNamespace()) == 0
