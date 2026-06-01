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
  import { negativeSpaceActive } from '$lib/state/tray.svelte';

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

  let negSpace = $derived(negativeSpaceActive());

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
  // statically per field so a low-coverage cell carries WP-003 §3.2's
  // framing in the publisher-side voice ("this is a publisher choice")
  // rather than the data-side voice ("we have no data"). Phase 122f
  // does not route this through the Content Catalog — keeping the
  // coupling tight to the field set the substrate enumerates.
  const FIELD_PROSE: Record<string, string> = {
    published_date:
      'this source does not emit machine-readable publication dates — temporal analyses must rely on heuristic htmldate inference or the fetch fallback.',
    modified_date:
      'this source does not emit modification timestamps — silent edits remain invisible at the metadata layer (cf. Phase 122d revision-archaeology sidecar).',
    author:
      'this source does not emit per-article authorship — author-level analyses are not comparable across sources whose authorship is asymmetric.',
    description:
      'this source does not emit article descriptions — abstracts must be derived from the full article body where required.',
    categories:
      'this source does not emit categorical taxonomies — topic comparisons cannot use publisher-declared sections from this source.',
    tags: 'this source does not emit free-form tags — tag-based clustering is sparser here than for sources that do.',
    section:
      'this source does not emit canonical section names — sectional analyses rely on URL-section heuristics for this source.',
    image_url:
      'this source does not emit canonical image references — image-driven analyses are systematically asymmetric.',
    article_type:
      'this source does not emit Schema.org article types — discourse-form analyses cannot rely on publisher-declared types here.',
    word_count:
      'this source does not emit pre-computed word counts — counts are derived from the cleaned body.',
    comment_count:
      'this source does not emit comment counts — engagement-volume comparisons exclude this source.',
    comment_url:
      'this source does not emit comment thread URLs — discussion follow-through is unobservable from metadata.',
    editor:
      'this source does not emit editorial responsibility — editor-level provenance analyses are systematically asymmetric.',
    reading_time_minutes:
      'this source does not emit reading-time hints — must be derived from word count.',
    dateline_location:
      'this source does not emit datelines — geographic origin analyses are systematically asymmetric.',
    paywall_status:
      'this source does not declare paywall status — accessibility analyses cannot include this source authoritatively.',
    correction_notice:
      'this source does not emit correction notices — silent correction practices are unobservable at the metadata layer.',
    editorial_labels:
      'this source does not emit editorial labels (op-ed / commentary / news) — register analyses are systematically asymmetric.',
    external_citations:
      'this source does not emit external citation metadata — cross-source citation graphs exclude this source.',
    images:
      'this source does not emit per-image metadata — multimedia provenance analyses are systematically asymmetric.',
    social_share_counts:
      'this source does not emit social share counts — virality comparisons exclude this source.',
    revision_date:
      'this source does not emit explicit revision dates — silent revisions remain invisible (cf. Phase 122d).'
  };

  function fieldProse(name: string, sourceName: string): string {
    const base =
      FIELD_PROSE[name] ??
      `this source does not emit \`${name}\` — analyses that require this field exclude it for this source.`;
    return `${sourceName} ${base}`;
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
    <h2 id="metadata-coverage-heading" class="section-title">Metadata coverage</h2>
    <p class="section-lede">
      Per-source emission posture across the Tier-B / Tier-C fields the WebAdapter records. WP-003
      §3.2 frames metadata-richness asymmetry as a structural bias; cells flagged as
      <em>structurally absent</em> reflect the publisher's choice not to emit, not sampling variance.
    </p>
  </header>

  {#if query.isPending}
    <p class="muted" aria-busy="true">Loading metadata coverage…</p>
  {:else if query.isError || query.data?.kind !== 'success'}
    <p class="muted">Coverage matrix unavailable.</p>
  {:else if query.data.data.sources.length === 0}
    <p class="muted">No sources in the probe scope.</p>
  {:else}
    <ul class="mc-source-grid" role="list">
      {#each query.data.data.sources as src (src.name)}
        {@const ordered = orderFields(src.fields)}
        {@const populatedCount = ordered.filter((f) => (f.populationRate ?? 0) > 0).length}
        <li class="mc-source-card">
          <details class="mc-source-details">
            <summary class="mc-source-summary">
              <span class="mc-summary-glyph" aria-hidden="true">›</span>
              <h3 class="mc-source-name">{src.name}</h3>
              <span
                class="mc-summary-meta"
                title="Fields this source actually populates (≥1 article) out of the full Tier-B/C set"
              >
                {populatedCount} / {ordered.length} field{ordered.length === 1 ? '' : 's'} populated
              </span>
            </summary>
            {#if ordered.length === 0}
              <p class="muted small">No coverage observations recorded yet for this source.</p>
            {:else}
              <ul class="mc-field-list" role="list">
                {#each ordered as f (f.field)}
                  {@const methods = methodKeys(f.byMethod)}
                  {@const populated = Math.round((f.populationRate ?? 0) * 100)}
                  <li
                    class="mc-field"
                    class:absent={f.structurallyAbsent}
                    class:absent-revealed={f.structurallyAbsent && negSpace}
                  >
                    <div class="mc-field-head">
                      <span class="mc-field-name" title={f.field}>{f.field}</span>
                      {#if f.structurallyAbsent}
                        <span class="mc-tag" aria-label="structurally absent">∅ absent</span>
                      {:else if f.totalArticles === 0}
                        <span class="mc-tag muted" aria-label="no observations">·</span>
                      {:else}
                        <span class="mc-rate" aria-label="population rate {populated} percent"
                          >{populated}%</span
                        >
                      {/if}
                    </div>

                    {#if f.structurallyAbsent && negSpace}
                      <p class="mc-prose">{fieldProse(f.field, src.name)}</p>
                    {:else if f.totalArticles > 0}
                      <div
                        class="mc-bar"
                        role="img"
                        aria-label="extraction-method distribution: {methods
                          .map((m) => `${m} ${f.byMethod[m] ?? 0}`)
                          .join(', ')}; null {f.byMethod.null ?? 0}"
                      >
                        {#each methods as m (m)}
                          {@const count = f.byMethod[m] ?? 0}
                          {@const w = (100 * count) / Math.max(1, f.totalArticles)}
                          <span
                            class="mc-bar-seg"
                            style:width="{w}%"
                            style:background={METHOD_COLOR[m] ?? '#888'}
                            title="{m}: {count}"
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
                      <span class="mc-meta">{f.totalArticles} articles</span>
                    {/if}
                  </li>
                {/each}
              </ul>
            {/if}
          </details>
        </li>
      {/each}
    </ul>

    {#if negSpace}
      <p class="mc-overlay-note">
        Negative Space overlay active — structurally-absent cells carry the methodological-register
        prose. Toggle off to collapse to compact rendering.
      </p>
    {/if}
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

  .mc-overlay-note {
    margin: var(--space-2) 0 0 0;
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    font-family: var(--font-mono);
  }
</style>
