<script lang="ts">
  // Per-source card in the Probe Dossier (Phase 106).
  // Shows: name, type, article counts, publication frequency,
  // etic classification (WP-001), emic designation/context, Silver eligibility.
  // "View articles" expands the ArticlePreviewList inline.
  // "Narrow scope" sets URL sourceId so lane views filter by this source.
  import type { ProbeDossierSourceDto, FetchContext } from '$lib/api/queries';
  import ArticlePreviewList from './ArticlePreviewList.svelte';
  import DiscoveryCoveragePanel from './DiscoveryCoveragePanel.svelte';
  // Phase 122k: url-state imports removed — SourceCard holds local state.
  import { getFunctionDef } from '$lib/discourse-function';
  import FunctionBadge from '$lib/components/base/FunctionBadge.svelte';

  interface Props {
    source: ProbeDossierSourceDto;
    ctx: FetchContext;
    windowStart: string;
    windowEnd: string;
  }

  let { source, ctx, windowStart, windowEnd }: Props = $props();

  // Function metadata sourced from $lib/discourse-function
  // (Phase 122h / ADR-033 §4 — single source of truth).

  // Phase 122k §6 finding — the card itself is now an expandable
  // container so the DF-Card body scales to many sources without
  // dominating the layout. Collapsed shows a compact one-line summary;
  // expanded reveals stats, classification, and the per-source detail
  // panels (articles, discovery coverage).
  let cardExpanded = $state(false);
  let articlesExpanded = $state(false);
  let discoveryExpanded = $state(false);

  // Phase 122k K2: scope-narrow-via-URL was the Phase-122h pattern; the
  // new ScopeEditor handles scope configuration, so this card is now
  // purely informational (no "Narrow scope" action).

  let primaryMeta = $derived(getFunctionDef(source.primaryFunction));
  let secondaryMeta = $derived(getFunctionDef(source.secondaryFunction));

  let freqDisplay = $derived(
    source.publicationFrequencyPerDay !== null && source.publicationFrequencyPerDay !== undefined
      ? source.publicationFrequencyPerDay.toFixed(1) + ' / day'
      : '—'
  );
</script>

<!-- eslint-disable svelte/no-navigation-without-resolve -- source.url and documentationUrl are external links -->
<article class="source-card" class:expanded={cardExpanded} aria-labelledby="sc-title-{source.name}">
  <button
    type="button"
    class="card-toggle"
    aria-expanded={cardExpanded}
    aria-controls="sc-body-{source.name}"
    onclick={() => (cardExpanded = !cardExpanded)}
  >
    <span class="chevron" class:expanded={cardExpanded} aria-hidden="true">›</span>
    <h3 id="sc-title-{source.name}" class="source-name">{source.name}</h3>
    <span class="type-badge">{source.type}</span>
    {#if source.silverEligible}
      <span
        class="silver-badge"
        aria-label="Silver-layer eligible"
        title="Silver-layer access approved (WP-006 §5.2)">Ag</span
      >
    {/if}
    <span class="toggle-summary" aria-hidden={cardExpanded ? 'true' : 'false'}>
      {source.articlesInWindow.toLocaleString()} in window · {freqDisplay}
    </span>
  </button>

  {#if cardExpanded}
    <div class="card-body" id="sc-body-{source.name}">
      {#if source.url}
        <a
          href={source.url}
          class="source-url"
          target="_blank"
          rel="noopener noreferrer"
          aria-label="Source URL: {source.url}"
        >
          {source.url}
        </a>
      {/if}

      <!-- Article counts -->
      <dl class="stats-row">
        <div class="stat">
          <dt>Total</dt>
          <dd>{source.articlesTotal.toLocaleString()}</dd>
        </div>
        <div class="stat">
          <dt>In window</dt>
          <dd>{source.articlesInWindow.toLocaleString()}</dd>
        </div>
        <div class="stat">
          <dt>Frequency</dt>
          <dd>{freqDisplay}</dd>
        </div>
      </dl>

      <!-- WP-001 etic/emic classification -->
      {#if primaryMeta || source.emicDesignation}
        <section class="classification" aria-label="WP-001 classification">
          {#if primaryMeta}
            <div class="etic-row">
              <span class="label-xs">Primary function</span>
              <FunctionBadge function={primaryMeta.key} size="sm" showLabel showInfo />
              {#if secondaryMeta}
                <span class="label-xs">·</span>
                <FunctionBadge function={secondaryMeta.key} size="sm" showLabel />
              {/if}
              <span
                class="provisional-hint"
                title="Source-level classification only — per-article discourse-function distribution (span) lands in Phase 122a.1"
              >
                (provisional)
              </span>
            </div>
          {/if}
          {#if source.emicDesignation}
            <p class="emic-designation">"{source.emicDesignation}"</p>
          {/if}
          {#if source.emicContext}
            <p class="emic-context">{source.emicContext}</p>
          {/if}
        </section>
      {/if}

      <!-- Actions -->
      <footer class="card-footer">
        <button
          type="button"
          class="action-btn"
          class:active={articlesExpanded}
          aria-expanded={articlesExpanded}
          aria-controls="articles-{source.name}"
          onclick={() => (articlesExpanded = !articlesExpanded)}
        >
          {articlesExpanded ? '↑ Hide articles' : '↓ View articles'}
          <span
            class="article-count-badge"
            class:zero={source.articlesInWindow === 0}
            title={source.articlesInWindow === 0 && source.articlesTotal > 0
              ? `0 articles in window · ${source.articlesTotal} total`
              : `${source.articlesInWindow} articles in window`}
          >
            {source.articlesInWindow}
          </span>
        </button>

        <button
          type="button"
          class="action-btn"
          class:active={discoveryExpanded}
          aria-expanded={discoveryExpanded}
          aria-controls="discovery-{source.name}"
          aria-label="{discoveryExpanded
            ? 'Hide'
            : 'Show'} per-channel discovery coverage for source: {source.name}"
          onclick={() => (discoveryExpanded = !discoveryExpanded)}
        >
          {discoveryExpanded ? '↑ Hide discovery' : '↓ Discovery coverage'}
        </button>

        {#if source.documentationUrl}
          <a
            href={source.documentationUrl}
            class="action-btn"
            target="_blank"
            rel="noopener noreferrer"
            aria-label="Open source dossier documentation">↗ Dossier</a
          >
        {/if}
      </footer>

      <!-- Article preview list (lazy, expanded on demand) -->
      {#if articlesExpanded}
        <div id="articles-{source.name}" class="article-list-slot">
          <ArticlePreviewList sourceId={source.name} {ctx} {windowStart} {windowEnd} />
        </div>
      {/if}

      <!-- Discovery coverage panel (Phase 122g — ADR-031). Lazy, expanded on demand. -->
      {#if discoveryExpanded}
        <div id="discovery-{source.name}" class="discovery-slot">
          <DiscoveryCoveragePanel sourceId={source.name} {ctx} />
        </div>
      {/if}
    </div>
  {/if}
</article>

<style>
  .source-card {
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    /* Subtle accent inherits from the parent DF-Card via --fn-color CSS
       custom property; outside a DF-Card the var is unset and we fall
       back to a neutral border-strong tone. */
    border-left: 3px solid var(--fn-color, var(--color-border-strong));
    border-radius: var(--radius-sm);
    display: flex;
    flex-direction: column;
    transition:
      border-color var(--motion-duration-fast) var(--motion-ease-standard),
      background-color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .source-card.expanded {
    background: var(--color-surface);
  }

  .provisional-hint {
    font-size: 10px;
    font-style: italic;
    color: var(--color-fg-subtle);
    margin-left: var(--space-1);
    cursor: help;
  }

  .card-toggle {
    appearance: none;
    background: transparent;
    border: none;
    color: inherit;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    padding: var(--space-2) var(--space-3);
    text-align: left;
  }
  .card-toggle:hover,
  .card-toggle:focus-visible {
    background: color-mix(in srgb, var(--fn-color, var(--color-fg)) 6%, transparent);
  }

  .chevron {
    color: var(--color-fg-subtle);
    transition: transform var(--motion-duration-fast) var(--motion-ease-standard);
    display: inline-block;
    flex-shrink: 0;
  }
  .chevron.expanded {
    transform: rotate(90deg);
  }

  .toggle-summary {
    margin-left: auto;
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
  }

  .card-body {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    padding: var(--space-2) var(--space-3) var(--space-3) var(--space-3);
    border-top: 1px dashed var(--color-border);
  }

  .source-name {
    font-size: var(--font-size-base);
    font-weight: var(--font-weight-semibold);
    font-family: var(--font-mono);
    color: var(--color-fg);
    margin: 0;
  }

  .type-badge {
    font-size: var(--font-size-xs);
    padding: 1px var(--space-2);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-pill);
    color: var(--color-fg-muted);
    font-family: var(--font-mono);
    text-transform: lowercase;
  }

  .silver-badge {
    font-size: 11px;
    padding: 1px var(--space-2);
    background: rgba(126, 196, 160, 0.15);
    border: 1px solid #7ec4a0;
    border-radius: var(--radius-pill);
    color: #7ec4a0;
    font-family: var(--font-mono);
    font-weight: var(--font-weight-semibold);
    cursor: default;
  }

  .source-url {
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-fg-subtle);
    text-decoration: none;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .source-url:hover {
    color: var(--color-accent);
  }

  .stats-row {
    display: flex;
    gap: var(--space-4);
    margin: 0;
  }

  .stat {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .stat dt {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-subtle);
  }

  .stat dd {
    font-size: var(--font-size-sm);
    font-family: var(--font-mono);
    color: var(--color-fg);
    margin: 0;
    font-weight: var(--font-weight-medium);
  }

  .classification {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding: var(--space-2) var(--space-3);
    background: var(--color-bg-elevated);
    border-radius: var(--radius-md);
    border-left: 2px solid var(--color-border-strong);
  }

  .etic-row {
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-2);
  }

  .label-xs {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-subtle);
  }

  .emic-designation {
    font-size: var(--font-size-sm);
    font-style: italic;
    color: var(--color-fg);
    margin: 0;
  }

  .emic-context {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
    margin: 0;
  }

  .card-footer {
    display: flex;
    gap: var(--space-2);
    flex-wrap: wrap;
    padding-top: var(--space-1);
    border-top: 1px solid var(--color-border);
    margin-top: auto;
  }

  .action-btn {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    padding: 3px var(--space-3);
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    cursor: pointer;
    text-decoration: none;
    transition: all var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .action-btn:hover,
  .action-btn:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .action-btn.active {
    color: var(--color-accent);
    border-color: var(--color-accent-muted);
    background: rgba(125, 220, 229, 0.08);
  }

  .article-count-badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 18px;
    height: 16px;
    padding: 0 4px;
    background: var(--color-bg-elevated);
    border-radius: var(--radius-pill);
    font-size: 10px;
    font-family: var(--font-mono);
    color: var(--color-fg-muted);
  }

  .article-count-badge.zero {
    opacity: 0.6;
    color: var(--color-fg-subtle);
  }

  .article-list-slot,
  .discovery-slot {
    border-top: 1px solid var(--color-border);
    padding-top: var(--space-3);
  }

  @media (prefers-reduced-motion: reduce) {
    .source-card,
    .action-btn {
      transition: none;
    }
  }
</style>
