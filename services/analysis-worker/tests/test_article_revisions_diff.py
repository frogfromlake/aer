"""Phase 122d.1 bugfix — headline-change rule + BUG-B sentinel contract.

These tests pin two invariants that previously had NO coverage, which is
how both bugs reached production:

1. **No spurious chain-head headline change.** The chain-head diff
   (revision_index=0) compares the current Silver body — wrapped with no
   ``<title>`` — against the oldest Wayback snapshot. The old rule
   ``prev != curr and bool(curr)`` fired on every chain-head pair because
   ``prev_headline`` is always empty, producing a bogus "− (empty) + <title>"
   on 109/109 chain-head rows. A headline change now requires a real title
   on BOTH sides.

2. **Sentinel serialisation contract.** ``SENTINEL_IDENTICAL_OP`` must decode
   to ``op == "identical"`` so the BFF (which decodes the op rather than
   string-matching) can detect "snapshots identical". The earlier byte-for-byte
   BFF compare missed the space Python's ``json.dumps`` emits.

``compute_diff`` only needs trafilatura for paragraph extraction; the headline
path falls back to a regex chain that works on raw strings, so these tests run
without the heavy NLP stack. When trafilatura is absent the body simply yields
no paragraph ops — irrelevant to the headline + sentinel assertions here.
"""

from __future__ import annotations

import json

from internal.article_revisions_diff import (
    SENTINEL_IDENTICAL_OP,
    compute_diff,
)

# Mimics `corpus._silver_text_to_html` — the chain-head "before" side. Note
# the deliberate absence of any <title>/<h1>/og:title element.
_SILVER_WRAPPER = "<!DOCTYPE html><html><body><p>Body paragraph one.</p></body></html>"
_WAYBACK_WITH_TITLE = (
    "<html><head><title>Astronauts head to space in 2027</title></head>"
    "<body><p>Body paragraph one.</p></body></html>"
)


def test_chainhead_pair_reports_no_headline_change() -> None:
    """prev (title-less Silver wrapper) vs curr (real title) → NOT a change."""
    result = compute_diff(_SILVER_WRAPPER, _WAYBACK_WITH_TITLE)
    assert result.headline_changed is False
    assert result.headline_before == ""
    assert result.headline_after == ""


def test_real_headline_change_detected_when_both_sides_have_titles() -> None:
    prev = "<html><head><title>Old headline</title></head><body><p>x</p></body></html>"
    curr = "<html><head><title>New headline</title></head><body><p>x</p></body></html>"
    result = compute_diff(prev, curr)
    assert result.headline_changed is True
    assert result.headline_before == "Old headline"
    assert result.headline_after == "New headline"


def test_identical_titles_are_not_a_headline_change() -> None:
    html = "<html><head><title>Same headline</title></head><body><p>x</p></body></html>"
    result = compute_diff(html, html)
    assert result.headline_changed is False


def test_empty_curr_headline_is_not_a_change() -> None:
    """Symmetry: an empty *curr* headline is also "unknown", never a change."""
    prev = "<html><head><title>Has a title</title></head><body><p>x</p></body></html>"
    curr = _SILVER_WRAPPER  # no title element
    result = compute_diff(prev, curr)
    assert result.headline_changed is False


def test_sentinel_decodes_to_identical_op() -> None:
    """The BFF detects the sentinel by decoding `op` — pin that contract."""
    encoded = json.dumps(SENTINEL_IDENTICAL_OP, ensure_ascii=False)
    assert json.loads(encoded)["op"] == "identical"


def test_identical_content_emits_only_the_sentinel() -> None:
    """No headline change + no paragraph ops ⇒ exactly the sentinel op."""
    html = "<html><head><title>T</title></head><body><p>same</p></body></html>"
    result = compute_diff(html, html)
    assert result.headline_changed is False
    assert len(result.diff_paragraphs) == 1
    assert json.loads(result.diff_paragraphs[0])["op"] == "identical"
