#!/usr/bin/env python3
"""Phase 122j J1 — bump contentVersion + lastReviewedDate on every YAML
file that was just patched with composition paragraphs. Also ensure the
WP-005 §6.2 anchor (joint corpus / cross-language methodology) is
listed under workingPaperAnchors for Episteme + Rhizome view_modes and
metrics that didn't already cite it.

Idempotent: only touches files whose long-text block contains the
phrase 'Composition —' OR 'Komposition —' AND whose contentVersion is
not already 'v2026-05-a'.
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path("services/bff-api/configs/content")
NEW_VERSION = '"v2026-05-a"'
NEW_DATE = '"2026-05-17"'
NEW_ANCHOR = '  - "WP-005 §6.2"'


def is_patched(text: str) -> bool:
    return ("Composition —" in text) or ("Komposition —" in text)


def bump_version_and_date(text: str) -> str:
    text = re.sub(r'^contentVersion:\s*"[^"]+"',
                  f"contentVersion: {NEW_VERSION}", text, count=1, flags=re.M)
    text = re.sub(r'^lastReviewedDate:\s*"[^"]+"',
                  f"lastReviewedDate: {NEW_DATE}", text, count=1, flags=re.M)
    return text


def ensure_anchor(text: str) -> str:
    if 'WP-005 §6.2' in text:
        return text
    # Append after the workingPaperAnchors block — easiest: just insert
    # the new anchor as the FINAL list entry by appending it at EOF if
    # the file ends with the anchors list.
    # Robust path: locate `workingPaperAnchors:` line, then append at
    # the end of consecutive `  - "..."` lines.
    lines = text.splitlines()
    out: list[str] = []
    in_anchors = False
    appended = False
    for i, line in enumerate(lines):
        if line.startswith("workingPaperAnchors:"):
            in_anchors = True
            out.append(line)
            continue
        if in_anchors and not appended:
            if line.startswith("  - "):
                out.append(line)
                # Look ahead — if next line is NOT a `  - ` entry, append now.
                if i + 1 >= len(lines) or not lines[i + 1].startswith("  - "):
                    out.append(NEW_ANCHOR)
                    appended = True
                continue
            else:
                # Section ended without us seeing `  - ` — handle anyway.
                out.append(NEW_ANCHOR)
                appended = True
                in_anchors = False
                out.append(line)
                continue
        out.append(line)
    if in_anchors and not appended:
        out.append(NEW_ANCHOR)
    result = "\n".join(out)
    if not result.endswith("\n"):
        result += "\n"
    return result


def process(path: Path) -> bool:
    text = path.read_text(encoding="utf-8")
    if not is_patched(text):
        return False
    if NEW_VERSION.strip('"') in text:
        # already bumped
        return False
    new = bump_version_and_date(text)
    new = ensure_anchor(new)
    if new != text:
        path.write_text(new, encoding="utf-8")
        return True
    return False


def main() -> int:
    if not ROOT.exists():
        print(f"ERR: {ROOT} not found", file=sys.stderr)
        return 2
    touched = 0
    for locale_dir in sorted(ROOT.iterdir()):
        if not locale_dir.is_dir() or locale_dir.name not in ("en", "de"):
            continue
        for sub in ("view_modes", "metrics"):
            d = locale_dir / sub
            if not d.is_dir():
                continue
            for f in sorted(d.glob("*.yaml")):
                if process(f):
                    touched += 1
    print(f"--- bumped {touched} files")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
