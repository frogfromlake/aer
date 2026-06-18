"""AĒR audit-source-discovery CLI — Phase 122g.

Operator-facing tool that probes a candidate source's homepage and
reports the discovery channels the publisher exposes. Two modes:

* **Onboarding mode** (default, no ``--sources-yaml`` argument).
  Prints a YAML-shaped ``discovery:`` block to stdout so the operator
  can paste the result into ``probes/<probe-id>/sources.yaml`` for a
  brand-new source.

* **Re-audit mode** (``--sources-yaml <path> --source <name>``).
  Loads the existing source's ``discovery:`` block from the YAML,
  compares it against the live audit, and prints an additive diff
  (only newly discovered URLs / channels are reported — entries
  present in the YAML but absent from the audit are NEVER flagged
  for removal, because publisher-surface disappearance is a
  methodological event handled by the underflow-alert telemetry,
  not a routine maintenance trigger). If the diff is non-empty the
  CLI prompts ``[y/N]`` and, on confirmation, updates the YAML
  in-place with ``ruamel.yaml`` (preserves comments + formatting)
  and writes a ``.bak`` backup.

This is the source-onboarding equivalent of the silently-installed
``ldap-search`` tool an operator uses to discover what an LDAP server
exposes before configuring an LDAP-bound app. The audit happens
*manually* (onboarding) or *periodically* (re-audit); runtime crawls
ALWAYS use the operator-curated configuration, never auto-discovery.
That separation is the load-bearing decision recorded in ADR-031:
auto-discovery as a config-authoring helper, not a runtime fallback.

Channels probed:
  * RSS / Atom feed auto-discovery via ``trafilatura.feeds.find_feed_urls``
    (``<link rel="alternate">`` + standard CMS paths). Trafilatura is
    optional — the audit gracefully degrades if it isn't installed.
  * XML sitemap auto-discovery via ``trafilatura.sitemaps.sitemap_search``
    (robots.txt + standard locations). Same optional-dep degradation.
  * Direct RSS-path probes — common publisher conventions
    (``/feed``, ``/rss.xml``, ``/atom.xml``, ``/index~rss2.xml`` etc.).
  * RSS-catalogue page parsing — publishers like bundesregierung
    expose a catalogue page at ``/service/newsletter-und-abos/...``
    whose ``<link rel="alternate">`` / ``<a href="*.xml">`` set
    enumerates several official feeds.
  * HTML sitemap probes — common publisher paths that surface a
    navigation index in HTML.
  * Date-indexed archive probes — common patterns publishers expose
    for date-walking.

Usage::

    # Onboarding a brand-new source (prints suggested YAML block)
    python audit_source.py <homepage_url>

    # Re-auditing an existing source (diff against sources.yaml, prompt)
    python audit_source.py <homepage_url> \\
        --sources-yaml probes/probe0/sources.yaml \\
        --source tagesschau

    # Re-auditing every source in a probe (loops, prompts per source)
    python audit_source.py --probe probes/probe0/sources.yaml

Cross-link: Mediacloud (https://search.mediacloud.org) maintains a
public registry of ~ 60,000 news sources with curated feed lists. If
the source already exists there, the CLI suggests importing.
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
from pathlib import Path
from typing import Optional


from audit_probe import (
    _A_HREF_RE,
    ARCHIVE_INDEX_CANDIDATES,
    _ASSET_EXTENSIONS,
    DEFAULT_TIMEOUT,
    DEFAULT_USER_AGENT,
    _extract_feed_links_from_catalogue,
    _FEED_HREF_RE,
    _FEED_LINK_REL_HREF_FIRST_RE,
    _FEED_LINK_REL_RE,
    _fetch_body,
    _fetch_homepage,
    _GENERATOR_META_RE,
    HTML_SITEMAP_CANDIDATES,
    _is_feed_like,
    _probe_http,
    _probe_rss_catalogues,
    _probe_rss_paths,
    RSS_CATALOGUE_CANDIDATES,
    RSS_FEED_CANDIDATES,
    _try_trafilatura_feeds,
    _try_trafilatura_sitemaps,
)
from audit_pattern import (
    cms_pattern_suggestions,
    CMS_PATTERN_TEMPLATES,
    _detect_cms,
    _extract_article_url_candidates,
    _extract_non_article_links,
    _infer_article_url_pattern,
    infer_safe_pattern,
    _validate_article_listing_page,
    validate_inferred_pattern,
)
from audit_datewalk import _verify_date_walker
from audit_core import audit_source, extract_discovered_urls
from audit_yaml import (
    apply_diff_to_yaml,
    diff_against_configured,
    _find_source_block,
    _format_yaml_suggestion,
    _load_sources_yaml,
    _prompt_yes_no,
    render_diff,
)
from audit_reaudit import _run_reaudit, _yaml_contains_edit_me, _format_edit_me_warning

logger = logging.getLogger(__name__)

__all__ = [
    "_A_HREF_RE",
    "ARCHIVE_INDEX_CANDIDATES",
    "_ASSET_EXTENSIONS",
    "DEFAULT_TIMEOUT",
    "DEFAULT_USER_AGENT",
    "_extract_feed_links_from_catalogue",
    "_FEED_HREF_RE",
    "_FEED_LINK_REL_HREF_FIRST_RE",
    "_FEED_LINK_REL_RE",
    "_fetch_body",
    "_fetch_homepage",
    "_GENERATOR_META_RE",
    "HTML_SITEMAP_CANDIDATES",
    "_is_feed_like",
    "_probe_http",
    "_probe_rss_catalogues",
    "_probe_rss_paths",
    "RSS_CATALOGUE_CANDIDATES",
    "RSS_FEED_CANDIDATES",
    "_try_trafilatura_feeds",
    "_try_trafilatura_sitemaps",
    "cms_pattern_suggestions",
    "CMS_PATTERN_TEMPLATES",
    "_detect_cms",
    "_extract_article_url_candidates",
    "_extract_non_article_links",
    "_infer_article_url_pattern",
    "infer_safe_pattern",
    "_validate_article_listing_page",
    "validate_inferred_pattern",
    "_verify_date_walker",
    "audit_source",
    "extract_discovered_urls",
    "apply_diff_to_yaml",
    "diff_against_configured",
    "_find_source_block",
    "_format_yaml_suggestion",
    "_load_sources_yaml",
    "_prompt_yes_no",
    "render_diff",
    "_run_reaudit",
    "_yaml_contains_edit_me",
    "_format_edit_me_warning",
    "cli",
]


def cli(argv: Optional[list[str]] = None) -> int:
    """CLI entrypoint for aer-audit-source: inventory a candidate source's
    discovery channels and print a sources.yaml stub. Returns a process exit code."""
    parser = argparse.ArgumentParser(
        prog="audit-source-discovery",
        description="Probe a candidate news source's discovery surfaces (Phase 122g).",
    )
    parser.add_argument(
        "homepage",
        nargs="?",
        help="The candidate source's homepage URL (e.g. https://www.tagesschau.de). Omit when using --probe.",
    )
    parser.add_argument(
        "--sources-yaml",
        type=Path,
        help="Path to the probe's sources.yaml. Activates re-audit / diff mode.",
    )
    parser.add_argument(
        "--source",
        help="Name of the source inside sources.yaml to re-audit (used with --sources-yaml).",
    )
    parser.add_argument(
        "--probe",
        type=Path,
        help="Path to a probe sources.yaml. Re-audits EVERY source in the probe; each source "
        "must declare `homepage_url:`. Prompts per source unless --yes is passed.",
    )
    parser.add_argument(
        "--yes",
        action="store_true",
        help="Auto-confirm all diffs (non-interactive). Use in CI / scripted runs.",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Show the diff but never write to YAML.",
    )
    parser.add_argument(
        "--verbose",
        action="store_true",
        help="Include non-200 probe results in the output (debugging only).",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Emit the raw audit report as JSON instead of the YAML suggestion "
        "(onboarding mode only — incompatible with --sources-yaml/--probe).",
    )
    parser.add_argument(
        "--timeout",
        type=float,
        default=DEFAULT_TIMEOUT,
        help=f"Per-probe HTTP timeout in seconds (default: {DEFAULT_TIMEOUT})",
    )
    args = parser.parse_args(argv)

    logging.basicConfig(level=logging.INFO, format="%(levelname)s %(name)s: %(message)s")

    # Mode 1: batch re-audit (--probe).
    if args.probe:
        if args.json or args.sources_yaml or args.source or args.homepage:
            print(
                "error: --probe is exclusive with --json/--sources-yaml/--source/<homepage>",
                file=sys.stderr,
            )
            return 2
        data = _load_sources_yaml(args.probe)
        exit_code = 0
        for src in data.get("sources") or []:
            name = src.get("name")
            homepage = src.get("homepage_url")
            if not name:
                continue
            if not homepage:
                print(
                    f"\n=== {name}: skipped (no `homepage_url:` declared in source block) ===",
                    file=sys.stderr,
                )
                continue
            rc = _run_reaudit(
                yaml_path=args.probe,
                source_name=name,
                homepage=homepage,
                timeout=args.timeout,
                verbose=args.verbose,
                auto_yes=args.yes,
                dry_run=args.dry_run,
            )
            if rc != 0:
                exit_code = rc  # only real errors (rc==2) propagate
        return exit_code

    # Mode 2: single re-audit (--sources-yaml + --source).
    if args.sources_yaml or args.source:
        if not (args.sources_yaml and args.source and args.homepage):
            print(
                "error: re-audit mode requires --sources-yaml, --source, AND a homepage argument.",
                file=sys.stderr,
            )
            return 2
        return _run_reaudit(
            yaml_path=args.sources_yaml,
            source_name=args.source,
            homepage=args.homepage,
            timeout=args.timeout,
            verbose=args.verbose,
            auto_yes=args.yes,
            dry_run=args.dry_run,
        )

    # Mode 3: onboarding (default — homepage positional argument only).
    if not args.homepage:
        parser.error("either <homepage> or --probe is required")

    try:
        report = audit_source(
            args.homepage,
            timeout=args.timeout,
            verbose=args.verbose,
        )
    except ValueError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2

    if args.json:
        print(json.dumps(report, indent=2, default=str))
    else:
        print(_format_yaml_suggestion(report))

    return 0


if __name__ == "__main__":
    sys.exit(cli())
