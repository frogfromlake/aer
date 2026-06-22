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

from dataclasses import dataclass
from typing import Optional


@dataclass
class ChannelStats:
    """Phase 148d (WP-007) — per-channel declared-inventory telemetry sink.

    A mutable sink threaded into each ``discover_*`` so the per-channel
    **declared denominator** — the publisher-advertised, in-window item
    count measured at the parse boundary, *before* AĒR's cross-channel
    dedup and URL/content filters — can be returned alongside the
    streamed entries without breaking the generator contract.

    ``declared`` is the completeness denominator (WP-007 §4.1):
    ``completeness = collected / declared``. ``indeterminate`` is set
    whenever ``declared`` is only a **lower bound** and therefore cannot
    be trusted as the full in-window inventory:

    * a fetch/parse error swallowed entries (fail-silent channel path),
    * a walk/fetch cap truncated the channel (``_MAX_SITEMAP_FETCHES``,
      archive depth), or
    * the channel surfaced advertised-but-undatable content (entries with
      no date in a windowed crawl), so we cannot prove the in-window count
      is complete.

    When ``indeterminate`` is True the BFF reports completeness as
    *indeterminate* (Negative Space), never a clean ratio and never 100 %
    (WP-007 §3, §5) — this is the structural answer to "are we *sure* we
    didn't overlook a page?": we do not assert certainty we lack.

    Left at the sentinel default (``declared is None``) by mocked test
    doubles that ignore the sink; the crawler falls back to
    ``declared = discovered`` in that case so telemetry never regresses to
    a spurious zero.
    """

    declared: Optional[int] = None
    indeterminate: bool = False

    def count(self, n: int = 1) -> None:
        """Tally ``n`` in-window declared items (lazily initialises declared)."""
        self.declared = (self.declared or 0) + n

    def mark_indeterminate(self) -> None:
        """Flag the declared count as a lower bound (error / cap / undated)."""
        self.indeterminate = True
        if self.declared is None:
            self.declared = 0


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
