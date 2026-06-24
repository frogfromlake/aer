<script lang="ts">
  // Phase 122f — per-source-per-field metadata coverage matrix on the
  // Probe Dossier (Surface II foundation, Brief §4.2.1). WP-003 §3.2
  // documents metadata-richness asymmetry as a structural bias; this
  // panel makes the asymmetry visible at runtime by reading the new
  // `/probes/{id}/metadata-coverage` endpoint and rendering a stacked
  // per-field cell that distinguishes "publisher chose not to emit"
  // (`structurallyAbsent: true`) from "we have no data on this yet".
  //
  // Negative-Space integration (Brief §7.7): when the Negative Space
  // overlay is active, structurally-absent fields render with the
  // methodology-register prose ("bundesregierung does not emit `author`
  // — this is a publisher choice, not a missing observation"). When
  // off, they collapse to the conventional dim placeholder. The visual
  // is the same as before for non-absent fields; the SEMANTIC
  // distinction is now structural, not visual.
  import { createQuery } from '@tanstack/svelte-query';
  import {
    probeMetadataCoverageQuery,
    type FetchContext,
    type MetadataCoverageResponseDto,
    type MetadataCoverageFieldDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import { m } from '$lib/paraglide/messages.js';
  import { sourceLabel } from '$lib/state/labels.svelte';

  interface Props {
    probeId: string;
    ctx?: FetchContext;
  }

  let { probeId, ctx = { baseUrl: '/api/v1' } }: Props = $props();

  const query = createQuery<
    QueryOutcome<MetadataCoverageResponseDto>,
    Error,
    QueryOutcome<MetadataCoverageResponseDto>
  >(() => {
    const o = probeMetadataCoverageQuery(ctx, probeId);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  // Field display order — Tier-B before Tier-C, mirroring the
  // worker's `metadata_coverage.COVERAGE_FIELDS` enumeration so the
  // panel ordering is the same as the data substrate.
  const FIELD_ORDER: ReadonlyArray<string> = [
    'published_date',
    'modified_date',
    'author',
    'description',
    'categories',
    'tags',
    'section',
    'image_url',
    'article_type',
    'word_count',
    'comment_count',
    'comment_url',
    'editor',
    'reading_time_minutes',
    'dateline_location',
    'paywall_status',
    'correction_notice',
    'editorial_labels',
    'external_citations',
    'images',
    'social_share_counts',
    'revision_date'
  ];

  // Extraction-method colours — keyed off the worker's
  // `web_meta.ALLOWED_EXTRACTION_METHODS` set. The literal-string
  // `"null"` is rendered with the Negative-Space hatch pattern so it is
  // distinguishable from "no observations" at a glance even when the
  // overlay is off.
  const METHOD_COLOR: Record<string, string> = {
    json_ld: '#5283b8',
    open_graph: '#7ec4a0',
    microdata: '#c8a85a',
    rdfa: '#9a8fb8',
    html_meta: '#5fa5c8',
    heuristic_htmldate: '#c89a5a',
    derived: '#aaaaaa',
    null: 'transparent'
  };

  // Methodology-register prose for the Negative-Space overlay. Bound
  // per field so a low-coverage cell carries WP-003 §3.2's framing in
  // the publisher-side voice ("this is a publisher choice") rather than
  // the data-side voice ("we have no data"). Phase 144 — sourced from
  // the per-locale Paraglide `source` catalogue (the field set is the
  // substrate enumeration; each key mirrors a coverage field).
  const FIELD_PROSE: Record<string, () => string> = {
    published_date: m.source_coverage_field_prose_published_date,
    modified_date: m.source_coverage_field_prose_modified_date,
    author: m.source_coverage_field_prose_author,
    description: m.source_coverage_field_prose_description,
    categories: m.source_coverage_field_prose_categories,
    tags: m.source_coverage_field_prose_tags,
    section: m.source_coverage_field_prose_section,
    image_url: m.source_coverage_field_prose_image_url,
    article_type: m.source_coverage_field_prose_article_type,
    word_count: m.source_coverage_field_prose_word_count,
    comment_count: m.source_coverage_field_prose_comment_count,
    comment_url: m.source_coverage_field_prose_comment_url,
    editor: m.source_coverage_field_prose_editor,
    reading_time_minutes: m.source_coverage_field_prose_reading_time_minutes,
    dateline_location: m.source_coverage_field_prose_dateline_location,
    paywall_status: m.source_coverage_field_prose_paywall_status,
    correction_notice: m.source_coverage_field_prose_correction_notice,
    editorial_labels: m.source_coverage_field_prose_editorial_labels,
    external_citations: m.source_coverage_field_prose_external_citations,
    images: m.source_coverage_field_prose_images,
    social_share_counts: m.source_coverage_field_prose_social_share_counts,
    revision_date: m.source_coverage_field_prose_revision_date
  };

  function fieldProse(name: string, sourceName: string): string {
    const known = FIELD_PROSE[name];
    if (known) return `${sourceName} ${known()}`;
    return m.source_coverage_prose_fallback({ source: sourceName, field: name });
  }

  function orderFields(fields: readonly MetadataCoverageFieldDto[]): MetadataCoverageFieldDto[] {
    const indexOf = (name: string): number => {
      const i = FIELD_ORDER.indexOf(name);
      return i < 0 ? FIELD_ORDER.length : i;
    };
    return [...fields].sort((a, b) => {
      const da = indexOf(a.field);
      const db = indexOf(b.field);
      if (da !== db) return da - db;
      // Forward-compatible: fields not in the canonical list keep
      // a stable alphabetical order at the end.
      return a.field.localeCompare(b.field);
    });
  }

  function methodKeys(byMethod: Record<string, number>): string[] {
    return Object.keys(byMethod)
      .filter((k) => k !== 'null')
      .sort();
  }
</script>

<section class="metadata-coverage" aria-labelledby="metadata-coverage-heading">
  <header class="mc-header">
    <h2 id="metadata-coverage-heading" class="section-title">{m.source_coverage_heading()}</h2>
    <p class="section-lede">{m.source_coverage_lede()}</p>
  </header>

  {#if query.isPending}
    <p class="muted" aria-busy="true">{m.source_coverage_loading()}</p>
  {:else if query.isError || query.data?.kind !== 'success'}
    <p class="muted">{m.source_coverage_unavailable()}</p>
  {:else if query.data.data.sources.length === 0}
    <p class="muted">{m.source_coverage_no_sources()}</p>
  {:else}
    <ul class="mc-source-grid" role="list">
      {#each query.data.data.sources as src (src.name)}
        {@const ordered = orderFields(src.fields)}
        {@const populatedCount = ordered.filter((f) => (f.populationRate ?? 0) > 0).length}
        <li class="mc-source-card">
          <details class="mc-source-details">
            <summary class="mc-source-summary">
              <span class="mc-summary-glyph" aria-hidden="true">›</span>
              <h3 class="mc-source-name">{sourceLabel(src.name)}</h3>
              <span class="mc-summary-meta" title={m.source_coverage_summary_meta_title()}>
                {(ordered.length === 1
                  ? m.source_coverage_summary_meta_one
                  : m.source_coverage_summary_meta_other)({
                  populated: populatedCount,
                  total: ordered.length
                })}
              </span>
            </summary>
            {#if ordered.length === 0}
              <p class="muted small">{m.source_coverage_no_observations()}</p>
            {:else}
              <ul class="mc-field-list" role="list">
                {#each ordered as f (f.field)}
                  {@const methods = methodKeys(f.byMethod)}
                  {@const populated = Math.round((f.populationRate ?? 0) * 100)}
                  <li
                    class="mc-field"
                    class:absent={f.structurallyAbsent}
                    class:absent-revealed={f.structurallyAbsent}
                    class:constant={f.constant}
                  >
                    <div class="mc-field-head">
                      <span class="mc-field-name" title={f.field}>{f.field}</span>
                      {#if f.structurallyAbsent}
                        <span class="mc-tag" aria-label={m.source_coverage_field_absent_aria()}
                          >{m.source_coverage_field_absent_tag()}</span
                        >
                      {:else if f.constant}
                        <span
                          class="mc-tag mc-tag-constant"
                          title={m.source_coverage_field_constant_title({
                            value: f.constantValue ?? ''
                          })}
                          aria-label={m.source_coverage_field_constant_aria({
                            value: f.constantValue ?? ''
                          })}>{m.source_coverage_field_constant_tag()}</span
                        >
                      {:else if f.totalArticles === 0}
                        <span
                          class="mc-tag muted"
                          aria-label={m.source_coverage_field_no_obs_aria()}>·</span
                        >
                      {:else}
                        <span
                          class="mc-rate"
                          aria-label={m.source_coverage_field_rate_aria({ percent: populated })}
                          >{populated}%</span
                        >
                      {/if}
                    </div>

                    {#if f.structurallyAbsent}
                      <p class="mc-prose">{fieldProse(f.field, src.name)}</p>
                    {:else if f.totalArticles > 0}
                      <div
                        class="mc-bar"
                        role="img"
                        aria-label={m.source_coverage_field_bar_aria({
                          methods: methods.map((mk) => `${mk} ${f.byMethod[mk] ?? 0}`).join(', '),
                          nullCount: f.byMethod.null ?? 0
                        })}
                      >
                        {#each methods as mk (mk)}
                          {@const count = f.byMethod[mk] ?? 0}
                          {@const w = (100 * count) / Math.max(1, f.totalArticles)}
                          <span
                            class="mc-bar-seg"
                            style:width="{w}%"
                            style:background={METHOD_COLOR[mk] ?? '#888'}
                            title="{mk}: {count}"
                          ></span>
                        {/each}
                        {#if (f.byMethod.null ?? 0) > 0}
                          {@const w = (100 * (f.byMethod.null ?? 0)) / Math.max(1, f.totalArticles)}
                          <span
                            class="mc-bar-seg mc-bar-seg-null"
                            style:width="{w}%"
                            title="null: {f.byMethod.null ?? 0}"
                          ></span>
                        {/if}
                      </div>
                      <span class="mc-meta"
                        >{m.source_coverage_field_articles({ count: f.totalArticles })}</span
                      >
                      {#if f.constant}
                        <p class="mc-prose mc-prose-constant">
                          {m.source_coverage_field_constant_prose({ value: f.constantValue ?? '' })}
                        </p>
                      {/if}
                    {/if}
                  </li>
                {/each}
              </ul>
            {/if}
          </details>
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  .metadata-coverage {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .mc-header {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .section-title {
    font-size: var(--font-size-xs);
    font-weight: var(--font-weight-semibold);
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    margin: 0;
  }

  .section-lede {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
    /* Full width per Finding 1.3 — short text wraps less, vertical
       space saved. */
    line-height: var(--line-height-loose);
  }

  /* Collapsed-by-default per-source cards (Finding 1.3) — too much
     vertical real estate at first paint, expand on demand. */
  .mc-source-details {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }

  .mc-source-summary {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    cursor: pointer;
    list-style: none;
  }

  .mc-source-summary::-webkit-details-marker {
    display: none;
  }

  .mc-source-summary:focus-visible {
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
    border-radius: var(--radius-md);
  }

  .mc-summary-glyph {
    color: var(--color-fg-subtle);
    transition: transform var(--motion-duration-fast) var(--motion-ease-standard);
    flex-shrink: 0;
  }

  details[open] > .mc-source-summary .mc-summary-glyph {
    transform: rotate(90deg);
  }

  .mc-summary-meta {
    margin-left: auto;
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
  }

  @media (prefers-reduced-motion: reduce) {
    .mc-summary-glyph {
      transition: none;
    }
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  .muted.small {
    font-size: var(--font-size-xs);
  }

  .mc-source-grid {
    list-style: none;
    padding: 0;
    margin: 0;
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(20rem, 1fr));
    gap: var(--space-3);
  }

  .mc-source-card {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: var(--color-surface);
    padding: var(--space-3);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .mc-source-name {
    margin: 0;
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    letter-spacing: var(--letter-spacing-tight);
  }

  .mc-field-list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .mc-field {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 4px 6px;
    border-radius: var(--radius-sm);
    border: 1px solid transparent;
  }

  .mc-field.absent {
    color: var(--color-fg-subtle);
  }

  .mc-field.absent-revealed {
    border-color: var(--color-fg-subtle);
    background: repeating-linear-gradient(
      45deg,
      transparent,
      transparent 4px,
      rgba(255, 255, 255, 0.03) 4px,
      rgba(255, 255, 255, 0.03) 8px
    );
    color: var(--color-fg);
  }

  .mc-field-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
  }

  .mc-field-name {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: inherit;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mc-rate {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg);
    flex-shrink: 0;
  }

  .mc-tag {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    padding: 1px 6px;
    border-radius: var(--radius-pill);
    border: 1px solid var(--color-fg-subtle);
    color: var(--color-fg-subtle);
    flex-shrink: 0;
  }

  .mc-tag.muted {
    border-color: var(--color-border);
    color: var(--color-fg-muted);
  }

  /* Task A — "constant → no signal" marker. Methodological, never a warning:
     a perceptually-neutral dim accent (ADR-039 METHODOLOGICAL-NOT-WARNING). */
  .mc-tag-constant {
    border-color: color-mix(in srgb, var(--color-fg-subtle) 45%, var(--color-border));
    color: var(--color-fg-muted);
    background: color-mix(in srgb, var(--color-fg-subtle) 8%, transparent);
  }

  .mc-prose-constant {
    font-style: normal;
    color: var(--color-fg-muted);
  }

  .mc-bar {
    display: flex;
    height: 4px;
    width: 100%;
    border-radius: 2px;
    overflow: hidden;
    background: var(--color-bg);
  }

  .mc-bar-seg {
    display: block;
    height: 100%;
  }

  .mc-bar-seg-null {
    background-image: repeating-linear-gradient(
      45deg,
      transparent,
      transparent 2px,
      rgba(255, 255, 255, 0.18) 2px,
      rgba(255, 255, 255, 0.18) 4px
    );
    background-color: rgba(255, 255, 255, 0.04);
  }

  .mc-meta {
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--color-fg-subtle);
  }

  .mc-prose {
    margin: 4px 0 0 0;
    font-size: var(--font-size-xs);
    color: var(--color-fg);
    line-height: var(--line-height-loose);
    font-style: italic;
  }
</style>
