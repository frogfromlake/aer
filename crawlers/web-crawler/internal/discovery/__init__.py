"""Phase 122g — discovery channel modules + shared safety contract.

Sentinel + exception used by the crawler to enforce a HARD STOP when
an operator commits a ``sources.yaml`` whose ``article_url_pattern``
still carries the placeholder written by the audit CLI (when the
strict auto-pattern inference safety gate rejected the inferred regex).

Without this hard stop, a forgotten ``EDIT-ME-...`` placeholder would
silently match zero article URLs at runtime — the channel would
appear configured but contribute nothing to the corpus, and the
under-ingestion would only surface after the two-consecutive-runs
underflow alert. The hard stop turns silent failure into loud
startup failure.
"""

from __future__ import annotations


EDIT_ME_SENTINEL = "EDIT-ME-REGEX-MATCHING-ARTICLE-URLS"


class DiscoveryConfigurationError(RuntimeError):
    """Raised when a ``discovery:`` block in sources.yaml is unsafe to
    use at runtime (e.g. an ``article_url_pattern`` is still the
    placeholder the audit CLI writes when it can't infer a safe regex).
    Caught at crawler startup so the operator sees a clear, actionable
    error message instead of silent zero-ingestion.
    """


def assert_pattern_usable(pattern: str, *, channel: str, where: str) -> None:
    """Raise :class:`DiscoveryConfigurationError` if ``pattern`` is the
    audit-CLI placeholder. Called by every discovery module that
    consumes an ``article_url_pattern`` field."""
    if not pattern:
        raise DiscoveryConfigurationError(
            f"{channel} entry at {where!r} has empty `article_url_pattern`. "
            "Set a publisher-specific regex (or remove the entry). "
            "Run `make audit-source HOMEPAGE=<source>` for a suggested pattern."
        )
    if EDIT_ME_SENTINEL in pattern:
        raise DiscoveryConfigurationError(
            f"{channel} entry at {where!r} still carries the audit-CLI "
            f"placeholder `{EDIT_ME_SENTINEL}`. This is NOT a valid "
            "article_url_pattern — the audit CLI could not auto-infer a "
            "safe regex and asked you to write one manually.\n"
            "\n"
            "  → Open the URL in a browser, sample 5–10 article URLs,\n"
            "    and derive a Python regex matching them.\n"
            "  → Or run `make audit-probe` again with `--verbose` to see\n"
            "    the rejected candidate pattern + sample URLs.\n"
            "  → Or remove the entry entirely if you don't need this channel.\n"
            "\n"
            "The crawler refuses to start until this is resolved (Phase 122g "
            "safety gate). Silent zero-ingestion on a misconfigured channel "
            "would be worse than a loud startup failure."
        )
