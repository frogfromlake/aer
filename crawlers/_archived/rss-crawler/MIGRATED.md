# Archived — superseded by `crawlers/web-crawler/` (Phase 122)

The standalone Go RSS crawler that fed Probe 0 (`tagesschau`,
`bundesregierung`) from RSS summaries between Phase 39 and Phase 121
has been retired. Probe 0 now uses the generalised Python web crawler at
`crawlers/web-crawler/`, which fetches full article HTML instead of RSS
title-plus-description snippets.

This directory is preserved for git-history traceability. The retirement
is documented in:

- ROADMAP.md → Phase 122 (Probe 0 Migration — Full-Article Web Crawling)
- docs/arc42/09_architecture_decisions.md → ADR-028
- docs/operations/operations_playbook.md → "Web-crawl operations"
  (with a cross-link to "Archived procedures: RSS crawler" for one
  release cycle)

Per ROADMAP Phase 131, this directory is removed entirely after the
single-release-cycle deprecation window.

The code here is no longer included in `go.work`, `make lint`,
`make test`, or any CI pipeline. **Do not modify it.** New work goes to
`crawlers/web-crawler/`.
