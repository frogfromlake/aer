"""Re-audit flow + EDIT-ME warnings — extracted from audit_source.py (Phase 141)."""


import logging
import sys
from pathlib import Path


from audit_pattern import (cms_pattern_suggestions,infer_safe_pattern,validate_inferred_pattern)
from audit_core import (audit_source,extract_discovered_urls)
from audit_yaml import (apply_diff_to_yaml,diff_against_configured,_find_source_block,_load_sources_yaml,_prompt_yes_no,render_diff)

logger = logging.getLogger(__name__)

def _run_reaudit(
    *,
    yaml_path: Path,
    source_name: str,
    homepage: str,
    timeout: float,
    verbose: bool,
    auto_yes: bool,
    dry_run: bool,
    out=sys.stdout,
) -> int:
    """Re-audit one source: run audit, diff against YAML, optionally apply.

    Returns 0 for any valid operator workflow outcome (no diff,
    dry-run, declined, applied). Returns 2 only for actual errors
    (source missing, audit raised, IO failure). Operator decisions are
    not errors — they're the entire point of the y/N prompt.
    """
    data = _load_sources_yaml(yaml_path)
    source_block = _find_source_block(data, source_name)
    if source_block is None:
        print(
            f"error: source {source_name!r} not found in {yaml_path}",
            file=sys.stderr,
        )
        return 2

    try:
        report = audit_source(homepage, timeout=timeout, verbose=verbose)
    except ValueError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2

    discovered = extract_discovered_urls(report)
    configured = source_block.get("discovery") or {}
    diff = diff_against_configured(discovered, configured)
    has_changes = any(diff[k] for k in diff)

    # Build pattern-inference sample map per URL — pulled from the
    # html_sitemap_candidates / archive_index_candidates the audit
    # already populated. Used by apply_diff_to_yaml's safe-pattern gate.
    pattern_samples: dict[str, dict[str, list[str]]] = {}
    for hit in report.get("html_sitemap_candidates") or []:
        if hit.get("skipped"):
            continue
        pattern_samples[hit["url"]] = {
            "articles": hit.get("article_url_sample") or [],
            "non_articles": hit.get("non_article_url_sample") or [],
        }
    for hit in report.get("archive_index_candidates") or []:
        if hit.get("skipped"):
            continue
        tmpl = hit.get("url_template", "")
        if tmpl.startswith("/"):
            full_template = report.get("origin", "") + tmpl
        else:
            full_template = tmpl
        pattern_samples[full_template] = {
            "articles": hit.get("article_url_sample") or [],
            "non_articles": hit.get("non_article_url_sample") or [],
        }

    print(f"\n=== {source_name} ({homepage}) ===", file=out)
    print(render_diff(diff, color=out.isatty() if hasattr(out, "isatty") else False),
          file=out)

    if not has_changes:
        return 0

    # If html_sitemap / archive_index entries are in the diff, surface
    # the audit's verification context (sample URLs + auto-pattern
    # decision) so the operator sees exactly what will be written
    # before confirming.
    for url in (diff.get("html_sitemap_urls") or []) + (diff.get("archive_index_urls") or []):
        sample = pattern_samples.get(url) or {}
        articles = sample.get("articles") or []
        non_articles = sample.get("non_articles") or []
        if not articles:
            continue
        print(f"\n  for {url}:", file=out)
        print(f"    sample article links found on page ({len(articles)}):",
              file=out)
        for u in articles[:5]:
            print(f"      • {u}", file=out)
        if len(articles) > 5:
            print(f"      ... and {len(articles) - 5} more", file=out)
        if non_articles:
            print(f"    sample non-article links also on page ({len(non_articles)}):",
                  file=out)
            for u in non_articles[:3]:
                print(f"      • {u}", file=out)
        # Show the auto-pattern decision.
        pattern, diag = infer_safe_pattern(
            articles, non_articles, report.get("origin") or homepage,
        )
        if pattern:
            print(f"    auto-pattern: {pattern}", file=out)
            d = diag.get("diagnostic") or {}
            print(
                f"      validation: matched "
                f"{d.get('article_matched', '?')}/{d.get('article_total', '?')} "
                f"sample articles, "
                f"{d.get('non_article_matched', '?')}/{d.get('non_article_total', '?')} "
                f"non-article false-positives — will be written to YAML.",
                file=out,
            )
        else:
            print(
                f"    auto-pattern: NOT inferred — "
                f"{diag.get('rejected_reason', 'unknown')}",
                file=out,
            )
            if diag.get("inferred_pattern"):
                print(
                    f"      (candidate that was rejected: "
                    f"{diag['inferred_pattern']})",
                    file=out,
                )
            # Surface CMS-specific suggestions: when the audit detected
            # a CMS family from `<meta name="generator">`, evaluate the
            # corresponding canonical-URL patterns against the same
            # sample and show the operator their match scores. This is
            # the "you don't need to write the regex yourself" fallback.
            cms_family = report.get("cms_detected")
            suggestions = cms_pattern_suggestions(
                cms_family, report.get("origin") or homepage
            )
            if suggestions:
                print(
                    f"      Suggested patterns based on detected CMS "
                    f"({cms_family}):",
                    file=out,
                )
                for label, candidate in suggestions:
                    val = validate_inferred_pattern(
                        candidate,
                        article_urls=articles,
                        non_article_urls=non_articles,
                    )
                    print(
                        f"        • {label}",
                        file=out,
                    )
                    print(
                        f"            regex: {candidate}",
                        file=out,
                    )
                    if val["valid"]:
                        print(
                            f"            sample match: "
                            f"{val['article_matched']}/{val['article_total']} "
                            f"articles, "
                            f"{val['non_article_matched']}/{val['non_article_total']} "
                            f"non-article false-positives",
                            file=out,
                        )
                    else:
                        print(f"            (won't compile: {val['reason']})",
                              file=out)
                print(
                    "      → if one of these looks right, paste it into the "
                    "YAML manually (or write your own).",
                    file=out,
                )
            print(
                "      → EDIT-ME-REGEX-MATCHING-ARTICLE-URLS placeholder "
                "will be written; the crawler will REFUSE TO START until "
                "you replace it.",
                file=out,
            )
        print(
            "    verify yourself by opening the URL in a browser. If the "
            "shown sample links look like article URLs → accept; if "
            "they look like navigation / footer → decline.",
            file=out,
        )

    # Before the operator answers y/N, remind them of the two judgment
    # calls the tool cannot make: format-variant duplicates vs. genuine
    # coverage gain, and a manual catalogue-page / robots.txt spot-check
    # when something looks surprising. Kept terse — full guidance lives
    # in the onboarding-mode footer + docs/extending/add-a-source.md.
    print(
        "\nBefore accepting, judge each entry:\n"
        "  • Format duplicate? Multiple feeds on the same path stem\n"
        "    (.rss / ~atom.xml / ~rdf.xml) usually carry the SAME\n"
        "    articles in different XML dialects — accepting all costs\n"
        "    HTTP politeness budget for zero coverage gain.\n"
        "  • Genuine new surface? A feed on a distinct path / catalogue\n"
        "    page typically carries different content — worth adding.\n"
        "  • If unsure, check the publisher's footer / robots.txt\n"
        "    (`curl <homepage>/robots.txt`) or cross-reference\n"
        "    https://search.mediacloud.org.",
        file=out,
    )

    if dry_run:
        print("(dry-run — no changes written)", file=out)
        return 0
    if not auto_yes:
        if not _prompt_yes_no(
            f"Apply these additions to {yaml_path}?",
            default_no=True,
        ):
            print("declined — no changes written.", file=out)
            return 0
    added = apply_diff_to_yaml(
        yaml_path,
        source_name,
        diff,
        pattern_samples=pattern_samples,
        homepage_origin=report.get("origin") or homepage,
    )
    summary = ", ".join(f"{k}+{v}" for k, v in added.items() if v)
    print(f"wrote {summary} to {yaml_path} (backup: {yaml_path.with_suffix(yaml_path.suffix + '.bak')})",
          file=out)

    # Phase 122g — emit a HIGH-VISIBILITY red banner if any entry in
    # the just-written YAML still carries the EDIT-ME placeholder
    # (i.e. the strict auto-pattern gate rejected the inferred regex).
    # Combined with the runtime hard-stop in `internal/discovery/` the
    # operator gets two independent safety nets against silent
    # zero-ingestion: a loud CLI banner now, and a refuse-to-start at
    # the next `make crawl-<probe-id>`.
    if _yaml_contains_edit_me(yaml_path):
        print(_format_edit_me_warning(yaml_path), file=out)
    return 0


def _yaml_contains_edit_me(yaml_path: Path) -> bool:
    """Return True if the YAML file contains the audit-CLI placeholder.
    Cheap text scan — we don't need a YAML parser here."""
    try:
        with yaml_path.open("r", encoding="utf-8") as fh:
            return "EDIT-ME-REGEX-MATCHING-ARTICLE-URLS" in fh.read()
    except OSError:
        return False


def _format_edit_me_warning(yaml_path: Path) -> str:
    """High-visibility ANSI-red banner for an unresolved EDIT-ME-placeholder
    in sources.yaml. Mirrors the runtime hard-stop banner the crawler
    emits so the operator's mental model is consistent."""
    RED = "\033[1;31m"
    YEL = "\033[1;33m"
    RESET = "\033[0m"
    bar = "═" * 78
    return (
        f"\n{RED}{bar}{RESET}\n"
        f"{RED}  ⚠  ACTION REQUIRED — UNRESOLVED `article_url_pattern` in YAML  ⚠  {RESET}\n"
        f"{RED}{bar}{RESET}\n"
        f"\n"
        f"  {yaml_path} still contains one or more\n"
        f"  {YEL}article_url_pattern: EDIT-ME-REGEX-MATCHING-ARTICLE-URLS{RESET}\n"
        f"  entries. The audit CLI could not auto-infer a SAFE regex\n"
        f"  (100 % sample-recall + 0 % false-positives) and wrote the\n"
        f"  placeholder instead.\n"
        f"\n"
        f"  The crawler will REFUSE to start with this YAML\n"
        f"  (Phase 122g hard-stop in internal/discovery/__init__.py).\n"
        f"  This is intentional — silent zero-ingestion on a misconfigured\n"
        f"  channel would be far worse than a loud failure now.\n"
        f"\n"
        f"  To resolve, do ONE of the following:\n"
        f"    1. Open the relevant URL in a browser, sample 5–10 article\n"
        f"       URLs, derive a Python regex matching them, and replace\n"
        f"       the EDIT-ME placeholder.\n"
        f"    2. Re-run with `--verbose` to see which candidate pattern\n"
        f"       the audit considered + why it was rejected.\n"
        f"    3. Remove the offending html_sitemap_urls entry /\n"
        f"       archive_index block if you don't need that channel.\n"
        f"\n"
        f"  Backup of the previous YAML is at {yaml_path.with_suffix(yaml_path.suffix + '.bak')}.\n"
        f"{RED}{bar}{RESET}\n"
    )


