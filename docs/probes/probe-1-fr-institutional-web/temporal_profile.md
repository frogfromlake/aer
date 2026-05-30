# Probe 1 — Temporal Profile (WP-005)

> **Status:** Provisional engineering heuristic.

## Publication rates

| Source | Approx. articles / day | Articles / hour (avg) | `min_meaningful_resolution` |
| :--- | :--- | :--- | :--- |
| `franceinfo` | high (continuous newsroom) | ≥ 2 | `hourly` |
| `elysee` | low, event-driven bursts | < 1 | `daily` |

Rates are first-run estimates; refine from the per-channel telemetry after `make crawl-probe1`. `expected_floor_per_run` in `sources.yaml` is set conservatively (franceinfo 150, elysee 5) and should be tuned from the empirical baseline.

## Derivation

The heuristic divides the average publication rate by the bucket width and selects the coarsest resolution at which an average bucket still holds ≥ 1 document (WP-005 §3.3):

- **franceinfo** — a continuous national newsroom averages well above one document per hour, so `hourly` is meaningful (sub-hour buckets are noise-dominated).
- **Élysée** — the Présidence publishes in event-driven bursts around announcements, ceremonies, and diplomacy; hourly buckets average far below one document, so `daily` is the finest meaningful resolution.

## Cross-probe note — cadence asymmetry is signal

The volume gap between a 24/7 newsroom (franceinfo, tagesschau) and burst-driven institutional channels (Élysée, bundesregierung) is **institutional communication cadence**, not a defect (see `bias_assessment.md` #6). It is why cross-frame count comparisons require normalization or the refusal surface, and why per-function baselines are deliberately not populated at low N (Phase 124).

## Discovery surface

- **franceinfo:** dated `sitemap_news.xml` (≈ 500 recent items, 100 % `<lastmod>`) + `titres.rss`. Media sub-sitemaps (audio/video/TV-replay) are technically filtered.
- **Élysée:** `/feed` RSS **only**. The `sitemap.publication.xml` (≈ 15 300 URLs to 1998) is deliberately NOT used: all its `<lastmod>` values are the nightly sitemap-regeneration time (audited: every entry stamped within a 3-second window), not article dates, so under strict-lastmod it would back-fill the whole archive and break the probe's uniform temporal horizon — same failure class as Probe-0 bundesregierung. Non-FR locale mirrors (`/en/`, `/es/`, `/de/`) excluded.

franceinfo's news sitemap carries real per-article `<lastmod>` dates, so `sitemap_strict_lastmod: true` filters it cleanly on the rolling 7-day watermark. elysee's publication sitemap does NOT (its `<lastmod>` is the nightly regeneration time), so elysee is collected via RSS instead — two opposite manifestations of the same "sitemap lastmod is unreliable" problem Probe 0 first hit with bundesregierung (100 % undated).

## Cultural calendar

`configs/cultural_calendars/fr.yaml` (Phase 123) — public holidays, presidential/legislative elections, media events (Cannes, Roland-Garros, Fête de la Musique), and commemorations. Structurally parallel to `de.yaml`; this parity is a precondition for the Phase-124 cross-probe temporal equivalence grant. Analyst lookup only — not consumed by any service in the POC.
