# Probe 1 — Platform Bias Assessment (WP-003)

> **Status:** Provisional engineering assessment.

## Platform: Public Web (Phase 122)

| Field | Value | Rationale |
| :--- | :--- | :--- |
| `platform_type` | `web` | Raw HTML fetched from public web pages (ADR-028). |
| `access_method` | `public_web` | No auth, no paywall on the indexed surface; polite crawl honouring robots.txt. |
| `visibility_mechanism` | `editorial_sitemap_and_rss` | Articles surface via editor-curated sitemaps + RSS — editorial, not algorithmic ranking. |
| `moderation_context` | `editorial` | Editorially curated before publication. |
| `engagement_data_available` | `false` | No likes/shares/views ingested. |
| `account_metadata_available` | `false` | No author-account follower/verification data ingested. |

## Structural biases

1. **Milieu bias.** Probe 1 captures exclusively **institutional and editorial voice** (a public broadcaster + the Presidency). It does not represent French public opinion, grassroots discourse, social media, or any demographic. A documented parameter, not a defect.
2. **Editorial curation bias.** All content has passed an editorial process; the corpus excludes unedited user-generated content and spontaneous reaction.
3. **Function coverage bias.** Only EA and PL are represented; Cohesion & Identity and Subversion & Friction are absent — by construction, symmetric with Probe 0.
4. **Source-selection bias — Cloudflare-driven (collection-method).** The institutionally exact analogue of Probe 0's bundesregierung is the SIG government portal (`gouvernement.fr` → `info.gouv.fr`). It sits behind an active Cloudflare JS bot-challenge and is **not collectable** by AĒR's polite, JS-free crawler. `elysee.fr` was selected instead. This substitutes the head of **state** for the head of **government** as the PL source. The substitution is disclosed, not silent; it is a property of the observation, recorded so cross-probe comparison is interpreted correctly.
5. **PL-locus asymmetry (cross-cultural, WP-004).** Following from #4 and the French semi-presidential constitution, the locus of executive Power-Legitimation discourse is the head of state in France vs. the head of government in Germany. This is a comparative finding about discourse structure, not measurement error.
6. **Publication-volume asymmetry.** franceinfo is a continuous high-volume newsroom; the Élysée publishes in low, event-driven bursts. Across probes, bundesregierung (≈5/day) and the Élysée differ sharply from franceinfo and tagesschau. **Volume reflects institutional communication cadence — it is signal, not noise.** It does mean absolute article/entity counts are not comparable across sources or probes of different cadence: count-based cross-frame comparison requires `?normalization=zscore` (which the BFF refuses for scaled/intensive metrics and for ungranted equivalence) or the refusal surface. Per-article (intensive) metrics like sentiment are volume-independent.
7. **Discovery-surface asymmetry.** Each source exposes different discovery channels (franceinfo: dated news sitemap + RSS; elysee: RSS feed only). elysee's publication sitemap is deliberately not used: every `<lastmod>` is the nightly sitemap-regeneration time, not the article date, so honouring it under strict-lastmod would back-fill the archive to 1998 and break the probe-level uniform temporal horizon (same failure class as bundesregierung's unreliable sitemap). Coverage differences are caught at runtime by per-channel telemetry (`GET /api/v1/sources/{id}/discovery-coverage`).
8. **Media-format filtering (technical).** franceinfo's audio (`/replay-radio/`) and TV-replay (`/replay-jt/`) surfaces are excluded as non-text media — content-type filtering, not editorial/topic gating (WP-006 §3).

## Per-source notes

**franceinfo.** Public broadcaster; high continuous volume; collected via dated news sitemap + RSS on the canonical `franceinfo.fr` (francetvinfo.fr redirects here). The structural twin of tagesschau.

**Élysée.** Head-of-state communication; low, event-driven volume; collected via its RSS feed only (the publication sitemap's `<lastmod>` is the regeneration time → unusable for the rolling window, would back-fill to 1998; see #7). Selected over the bot-walled `info.gouv.fr` (#4). The structural twin of bundesregierung in function, with the locus caveat (#5).

## Implications for metric interpretation

- Sentiment and other per-article metrics are comparable across sources; **counts are not** without normalization.
- Cross-probe comparison is valid only where an equivalence grant exists (Phase 124: temporal Level-1 only at first); sentiment-level cross-probe equivalence is out of scope.
- Every metric reports `validation_status = unvalidated`.
