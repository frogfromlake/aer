<script lang="ts">
  // ProbeDossierView — the reusable methodological body of a probe dossier
  // (Phase 127). Extracted from reflection/probe/[id]/+page.svelte so the
  // singular page and the /reflection/probes aggregate render identical content
  // from one source. Owns the function-coverage, source list, and WP cross-link
  // regions; the page shell owns the title/eyebrow header and the back chrome.
  import type { ProbeDossierDto } from '$lib/api/queries';
  import { m } from '$lib/paraglide/messages.js';
  import { formatNumber } from '$lib/localization/format';

  interface Props {
    dossier: ProbeDossierDto;
  }
  let { dossier }: Props = $props();

  // Discourse-function keys are the load-bearing identifiers (matched against
  // covered functions); the label/description are localized at render time.
  const FUNCTION_KEYS = [
    'epistemic_authority',
    'power_legitimation',
    'cohesion_identity',
    'subversion_friction'
  ] as const;

  function functionLabel(key: string): string {
    switch (key) {
      case 'epistemic_authority':
        return m.reflection_function_epistemic_authority_label();
      case 'power_legitimation':
        return m.reflection_function_power_legitimation_label();
      case 'cohesion_identity':
        return m.reflection_function_cohesion_identity_label();
      case 'subversion_friction':
        return m.reflection_function_subversion_friction_label();
      default:
        return key;
    }
  }

  function functionDescription(key: string): string {
    switch (key) {
      case 'epistemic_authority':
        return m.reflection_function_epistemic_authority_desc();
      case 'power_legitimation':
        return m.reflection_function_power_legitimation_desc();
      case 'cohesion_identity':
        return m.reflection_function_cohesion_identity_desc();
      case 'subversion_friction':
        return m.reflection_function_subversion_friction_desc();
      default:
        return '';
    }
  }

  const coveredFunctions = $derived(dossier.sources.map((s) => s.primaryFunction).filter(Boolean));
</script>

<p class="probe-sub">{m.reflection_probe_sub()}</p>

<!-- Function coverage -->
<section class="coverage-section" aria-labelledby="coverage-heading-{dossier.probeId}">
  <h3 id="coverage-heading-{dossier.probeId}" class="section-title">
    {m.reflection_probe_coverage_heading()}
  </h3>
  <p class="section-body">
    {m.reflection_probe_coverage_body_pre()}
    <strong>{dossier.functionCoverage.covered}</strong>
    {m.reflection_probe_coverage_body_of()}
    <strong>{dossier.functionCoverage.total}</strong>
    {m.reflection_probe_coverage_body_post()}
    <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
    <a href="/reflection/wp/wp-001?section=3" class="wp-link">WP-001 §3</a>.
  </p>
  <ul class="function-list" role="list">
    {#each FUNCTION_KEYS as key (key)}
      {@const isCovered = coveredFunctions.includes(key as (typeof coveredFunctions)[number])}
      <li class="function-item" class:covered={isCovered} class:uncovered={!isCovered}>
        <span class="function-indicator" aria-hidden="true">{isCovered ? '●' : '○'}</span>
        <div>
          <p class="function-label">{functionLabel(key)}</p>
          <p class="function-desc">{functionDescription(key)}</p>
        </div>
      </li>
    {/each}
  </ul>
</section>

<!-- Sources -->
<section class="sources-section" aria-labelledby="sources-heading-{dossier.probeId}">
  <h3 id="sources-heading-{dossier.probeId}" class="section-title">
    {m.reflection_probe_sources_heading()}
  </h3>
  <ul class="source-list" role="list">
    {#each dossier.sources as source (source.name)}
      <li class="source-item">
        <div class="source-head">
          <code class="source-id">{source.name}</code>
          {#if source.primaryFunction}
            <span class="source-fn">{functionLabel(source.primaryFunction)}</span>
          {/if}
        </div>
        {#if source.emicDesignation}
          <p class="source-meta">
            <span class="meta-label">{m.reflection_probe_source_emic_designation()}</span>
            {source.emicDesignation}
          </p>
        {/if}
        {#if source.emicContext}
          <p class="source-meta">
            <span class="meta-label">{m.reflection_probe_source_emic_context()}</span>
            {source.emicContext}
          </p>
        {/if}
        <p class="source-meta">
          <span class="meta-label">{m.reflection_probe_source_articles_in_window()}</span>
          {formatNumber(source.articlesInWindow)}
        </p>
      </li>
    {/each}
  </ul>
</section>

<!-- Methodological context cross-links -->
<nav class="methodology-links" aria-label={m.reflection_probe_methodology_context_aria()}>
  <h3 class="section-title">{m.reflection_probe_methodology_context_heading()}</h3>
  <ul class="link-list" role="list">
    <li>
      <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
      <a href="/reflection/wp/wp-001?section=5" class="ctx-link"
        >{m.reflection_probe_ctx_wp001_5()}</a
      >
    </li>
    <li>
      <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
      <a href="/reflection/wp/wp-001?section=6" class="ctx-link"
        >{m.reflection_probe_ctx_wp001_6()}</a
      >
    </li>
    <li>
      <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
      <a href="/reflection/wp/wp-003?section=6" class="ctx-link"
        >{m.reflection_probe_ctx_wp003_6()}</a
      >
    </li>
    <li>
      <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
      <a href="/reflection/wp/wp-006?section=5" class="ctx-link"
        >{m.reflection_probe_ctx_wp006_5()}</a
      >
    </li>
  </ul>
</nav>

<style>
  .probe-sub {
    font-size: var(--font-size-base);
    line-height: var(--line-height-loose);
    color: var(--color-fg-muted);
    margin: 0;
  }

  .section-title {
    font-size: var(--font-size-lg);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    margin: 0 0 var(--space-4);
  }

  .section-body {
    font-size: var(--font-size-base);
    line-height: var(--line-height-loose);
    color: var(--color-fg);
    margin: 0 0 var(--space-4);
  }

  .wp-link {
    color: var(--color-accent);
    font-family: var(--font-mono);
    font-size: 0.875em;
    text-decoration: none;
  }

  .wp-link:hover {
    text-decoration: underline;
  }

  /* Coverage */
  .function-list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .function-item {
    display: flex;
    gap: var(--space-3);
    padding: var(--space-3);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    align-items: flex-start;
  }

  .function-item.covered {
    background: rgba(126, 196, 160, 0.08);
    border-color: rgba(126, 196, 160, 0.3);
  }

  .function-item.uncovered {
    opacity: 0.55;
  }

  .function-indicator {
    font-size: var(--font-size-xs);
    color: var(--color-status-validated);
    flex-shrink: 0;
    padding-top: 2px;
  }

  .function-item.uncovered .function-indicator {
    color: var(--color-border-strong);
  }

  .function-label {
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    margin: 0 0 var(--space-1);
  }

  .function-desc {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    margin: 0;
    line-height: var(--line-height-loose);
  }

  /* Sources */
  .source-list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .source-item {
    padding: var(--space-4);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .source-head {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
  }

  .source-id {
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    color: var(--color-fg);
  }

  .source-fn {
    font-size: var(--font-size-xs);
    color: var(--color-accent-muted);
    padding: 1px var(--space-2);
    border: 1px solid var(--color-accent-muted);
    border-radius: var(--radius-pill);
  }

  .source-meta {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  .meta-label {
    font-weight: var(--font-weight-medium);
    color: var(--color-fg-muted);
    margin-right: var(--space-1);
  }

  /* Context links */
  .link-list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .ctx-link {
    color: var(--color-accent);
    text-decoration: none;
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
  }

  .ctx-link:hover {
    text-decoration: underline;
  }
</style>
