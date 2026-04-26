<script lang="ts">
  // Surface III — Working Paper reader (Phase 109).
  // Fetches the paper markdown from /content/papers/{id}.md, renders it
  // with the minimal GFM renderer, and presents it as long-form Distill-style
  // prose with:
  //   - Section nav in the ScopeBar breadcrumb
  //   - Inline Observable Plot cells after designated sections
  //   - Cross-reference links to /reflection/wp/{id}?section=…
  //   - Scroll-to-section when ?section= is in the URL
  import { page } from '$app/state';
  import { onMount } from 'svelte';
  import { ScopeBar } from '$lib/components/chrome';
  import InlineChart from '$lib/components/reflection/InlineChart.svelte';
  import { negativeSpaceActive } from '$lib/state/tray.svelte';
  import type { PageData } from './$types';

  interface Props {
    data: PageData;
  }
  let { data }: Props = $props();

  const paper = $derived(data.paper);
  const meta = $derived(paper?.meta ?? null);
  const rendered = $derived(paper?.rendered ?? null);
  const sections = $derived(rendered?.sections ?? []);
  const title = $derived(rendered?.title ?? meta?.shortTitle ?? 'Working Paper');

  // Active section from URL ?section= param
  const sectionParam = $derived(page.url.searchParams.get('section') ?? null);

  const negSpace = $derived(negativeSpaceActive());

  // Scroll to the requested section after mount
  onMount(() => {
    if (!sectionParam) return;
    const tryScroll = () => {
      // Try exact section number first, then by heading text
      const candidates = [
        `section-${sectionParam.replace(/\./g, '-')}`,
        `appendix-${sectionParam.toLowerCase()}`
      ];
      for (const id of candidates) {
        const el = document.getElementById(id);
        if (el) {
          el.scrollIntoView({ behavior: 'smooth', block: 'start' });
          return;
        }
      }
    };
    // Small delay to let the DOM settle after SSR hydration
    setTimeout(tryScroll, 80);
  });

  // Collect non-appendix and appendix sections for the TOC
  const mainSections = $derived(sections.filter((s) => !s.isAppendix && s.number));
  const appendixSections = $derived(sections.filter((s) => s.isAppendix));
</script>

<svelte:head>
  <title>AĒR — {title}</title>
</svelte:head>

<ScopeBar label="Reflection — Working Paper navigation">
  <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
  <a href="/reflection" class="breadcrumb-root" aria-label="Back to Reflection surface">
    Reflection
  </a>
  <span class="breadcrumb-sep" aria-hidden="true">›</span>
  <span class="breadcrumb-id" aria-current="page">
    {meta?.id.toUpperCase() ?? '…'}
  </span>
  {#if sectionParam}
    <span class="breadcrumb-sep" aria-hidden="true">›</span>
    <span class="breadcrumb-section">§{sectionParam}</span>
  {/if}
</ScopeBar>

<div class="wp-layout" class:neg-space={negSpace} id="main-reflection-wp">
  <!-- Table of contents / absence margin (sticky sidebar on wide screens) -->
  {#if mainSections.length > 0}
    <nav class="toc" aria-label="Table of contents">
      {#if negSpace}
        <!-- Surface III — Negative Space mode: scope boundary annotation (Phase 112).
             Absence-prose scrolls alongside the paper body per Design Brief §5.4. -->
        <aside class="absence-margin" aria-label="Scope boundary notes">
          <p class="absence-margin-heading">
            <span aria-hidden="true">∅</span> Scope boundary
          </p>
          <p class="absence-margin-text">
            This Working Paper documents the methodological boundaries of what AĒR observes. The
            system does not capture demographic variation, informal discourse, or sources outside
            active probes. Absence is data.
          </p>
          <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
          <a class="absence-margin-ref" href="/reflection/wp/wp-001?section=5.3">WP-001 §5.3</a>
        </aside>
        <hr class="absence-divider" aria-hidden="true" />
      {/if}
      <p class="toc-heading">Contents</p>
      <ol class="toc-list">
        {#each mainSections as s (s.id)}
          <li>
            <a href="#{s.id}" class="toc-link">
              <span class="toc-num">{s.number}.</span>
              {s.title}
            </a>
          </li>
        {/each}
        {#if appendixSections.length > 0}
          {#each appendixSections as s (s.id)}
            <li>
              <a href="#{s.id}" class="toc-link">
                <span class="toc-num">App. {s.number}</span>
                {s.title}
              </a>
            </li>
          {/each}
        {/if}
      </ol>
    </nav>
  {/if}

  <!-- Paper body -->
  <article class="paper" aria-labelledby="paper-title">
    {#if !paper}
      <div class="error-state">
        <h1 id="paper-title">Working Paper not found</h1>
        <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
        <a href="/reflection">← Back to Reflection</a>
      </div>
    {:else}
      <!-- Paper header -->
      <header class="paper-header">
        <p class="paper-series">AĒR Scientific Methodology Working Papers</p>
        <!-- eslint-disable-next-line svelte/no-at-html-tags -->
        <h1 id="paper-title" class="paper-title">{@html title}</h1>

        {#if meta}
          <dl class="paper-meta">
            <dt>Status</dt>
            <dd>{meta.status}</dd>
            <dt>Date</dt>
            <dd>{meta.date}</dd>
            {#if meta.depends.length > 0}
              <dt>Depends on</dt>
              <dd>
                {#each meta.depends as dep, i (dep)}
                  <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
                  <a href="/reflection/wp/{dep}" class="meta-link">{dep.toUpperCase()}</a>{i <
                  meta.depends.length - 1
                    ? ', '
                    : ''}
                {/each}
              </dd>
            {/if}
          </dl>
        {/if}

        {#if !rendered}
          <div class="load-error" role="alert">
            <p>Could not load paper content. Connect to the AĒR backend or check your network.</p>
          </div>
        {/if}
      </header>

      <!-- Sections -->
      {#if rendered}
        {#each sections as section (section.id)}
          <section
            class="paper-section"
            class:appendix={section.isAppendix}
            id={section.id}
            aria-labelledby="{section.id}-heading"
          >
            {#if section.number || section.title}
              <h2 id="{section.id}-heading" class="section-heading">
                {#if section.number && !section.isAppendix}
                  <span class="section-num">{section.number}.</span>
                {:else if section.isAppendix}
                  <span class="section-num">Appendix {section.number}</span>
                {/if}
                {section.title}
              </h2>
            {/if}

            <!-- Rendered markdown HTML -->
            <div class="section-body prose">
              <!-- eslint-disable-next-line svelte/no-at-html-tags -->
              {@html section.html}
            </div>

            <!-- Inline interactive cells — rendered after designated sections -->
            {#if meta}
              {#each meta.interactiveCells.filter((c) => c.afterSection === section.number) as cell (cell.cellId)}
                <InlineChart cellId={cell.cellId} />
              {/each}
            {/if}
          </section>
        {/each}
      {/if}

      <!-- Open questions link -->
      <footer class="paper-footer">
        <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
        <a href="/reflection/open-questions" class="footer-link"> ← All open research questions </a>
        {#if meta?.downstream && meta.downstream.length > 0}
          <span class="footer-downstream">
            Downstream:
            {#each meta.downstream as dep, i (dep)}
              <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
              <a href="/reflection/wp/{dep}" class="footer-link">{dep.toUpperCase()}</a>{i <
              meta.downstream.length - 1
                ? ', '
                : ''}
            {/each}
          </span>
        {/if}
      </footer>
    {/if}
  </article>
</div>

<style>
  .wp-layout {
    position: fixed;
    inset: 0;
    left: var(--rail-width);
    top: var(--scope-bar-height);
    right: var(--tray-right-edge, var(--tray-closed-width));
    display: grid;
    grid-template-columns: 200px 1fr;
    overflow: hidden;
    background: var(--color-bg);
  }

  @media (max-width: 900px) {
    .wp-layout {
      grid-template-columns: 1fr;
    }

    .toc {
      display: none;
    }
  }

  /* Table of contents */
  .toc {
    border-right: 1px solid var(--color-border);
    overflow-y: auto;
    padding: var(--space-5) var(--space-3);
    background: var(--color-bg-elevated);
  }

  /* Negative Space mode — TOC column highlights the scope boundary (Phase 112). */
  .neg-space .toc {
    border-right-color: rgba(82, 131, 184, 0.4);
    background: color-mix(in srgb, var(--color-bg-elevated) 90%, rgba(82, 131, 184, 0.15));
  }

  /* Absence margin annotation */
  .absence-margin {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-3);
    background: rgba(82, 131, 184, 0.07);
    border: 1px solid rgba(82, 131, 184, 0.25);
    border-radius: var(--radius-md);
    margin-bottom: var(--space-3);
  }

  .absence-margin-heading {
    font-size: var(--font-size-xs);
    text-transform: uppercase;
    letter-spacing: 0.07em;
    font-weight: var(--font-weight-semibold);
    color: rgba(82, 131, 184, 0.9);
    margin: 0;
  }

  .absence-margin-text {
    font-size: 11px;
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
    margin: 0;
  }

  .absence-margin-ref {
    font-size: 11px;
    font-family: var(--font-mono);
    color: var(--color-accent);
    text-decoration: none;
    border-bottom: 1px dotted var(--color-accent);
    align-self: flex-start;
  }

  .absence-margin-ref:hover,
  .absence-margin-ref:focus-visible {
    color: var(--color-fg);
    border-bottom-color: var(--color-fg);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .absence-divider {
    border: none;
    border-top: 1px solid rgba(82, 131, 184, 0.25);
    margin: 0 0 var(--space-3);
  }

  .toc-heading {
    font-size: var(--font-size-xs);
    text-transform: uppercase;
    letter-spacing: 0.07em;
    color: var(--color-fg-muted);
    font-weight: var(--font-weight-semibold);
    margin: 0 0 var(--space-3);
  }

  .toc-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .toc-link {
    display: flex;
    gap: var(--space-2);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    text-decoration: none;
    padding: 3px var(--space-2);
    border-radius: var(--radius-sm);
    line-height: 1.4;
    transition: all var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .toc-link:hover,
  .toc-link:focus-visible {
    color: var(--color-fg);
    background: var(--color-surface-hover);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .toc-num {
    color: var(--color-fg-subtle);
    font-family: var(--font-mono);
    font-size: 10px;
    flex-shrink: 0;
    padding-top: 1px;
  }

  /* Paper body */
  .paper {
    overflow-y: auto;
    padding: var(--space-7) var(--space-7) var(--space-9);
    max-width: 72ch;
    margin: 0 auto;
    width: 100%;
    box-sizing: border-box;
  }

  @media (max-width: 1100px) {
    .paper {
      padding: var(--space-5) var(--space-5) var(--space-7);
    }
  }

  .paper-series {
    font-size: var(--font-size-xs);
    text-transform: uppercase;
    letter-spacing: 0.07em;
    color: var(--color-fg-subtle);
    margin: 0 0 var(--space-3);
  }

  .paper-title {
    font-size: var(--font-size-2xl);
    font-weight: var(--font-weight-semibold);
    line-height: var(--line-height-tight);
    letter-spacing: var(--letter-spacing-tight);
    color: var(--color-fg);
    margin: 0 0 var(--space-5);
  }

  .paper-meta {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: var(--space-1) var(--space-4);
    font-size: var(--font-size-sm);
    margin: 0 0 var(--space-6);
    padding: var(--space-4);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }

  .paper-meta dt {
    color: var(--color-fg-muted);
    font-weight: var(--font-weight-medium);
  }

  .paper-meta dd {
    color: var(--color-fg);
    margin: 0;
  }

  .meta-link {
    color: var(--color-accent);
    text-decoration: none;
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
  }

  .meta-link:hover {
    text-decoration: underline;
  }

  .load-error {
    padding: var(--space-4);
    border: 1px dashed var(--color-border);
    border-radius: var(--radius-md);
    margin-bottom: var(--space-5);
  }

  .load-error p {
    margin: 0;
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
  }

  /* Section */
  .paper-section {
    margin-bottom: var(--space-8);
    scroll-margin-top: calc(var(--scope-bar-height) + var(--space-4));
  }

  .paper-section.appendix {
    border-top: 1px solid var(--color-border);
    padding-top: var(--space-6);
  }

  .section-heading {
    font-size: var(--font-size-xl);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    margin: 0 0 var(--space-5);
    line-height: var(--line-height-tight);
    display: flex;
    align-items: baseline;
    gap: var(--space-3);
  }

  .section-num {
    color: var(--color-fg-subtle);
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-regular);
    flex-shrink: 0;
  }

  /* Prose typography — governs all HTML generated by the markdown renderer */
  .prose :global(p) {
    font-size: var(--font-size-base);
    line-height: var(--line-height-loose);
    color: var(--color-fg);
    margin: 0 0 var(--space-4);
  }

  .prose :global(h3) {
    font-size: var(--font-size-md);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    margin: var(--space-6) 0 var(--space-3);
    line-height: var(--line-height-tight);
    scroll-margin-top: calc(var(--scope-bar-height) + var(--space-4));
  }

  .prose :global(h4) {
    font-size: var(--font-size-base);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    margin: var(--space-5) 0 var(--space-2);
  }

  .prose :global(strong) {
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
  }

  .prose :global(em) {
    font-style: italic;
  }

  .prose :global(code) {
    font-family: var(--font-mono);
    font-size: 0.875em;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 1px 4px;
  }

  .prose :global(pre) {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: var(--space-4);
    overflow-x: auto;
    margin: var(--space-4) 0;
  }

  .prose :global(pre code) {
    background: none;
    border: none;
    padding: 0;
    font-size: var(--font-size-sm);
  }

  .prose :global(blockquote) {
    border-left: 3px solid var(--color-border-strong);
    padding-left: var(--space-4);
    margin: var(--space-4) 0;
    color: var(--color-fg-muted);
  }

  .prose :global(blockquote p) {
    margin: 0;
  }

  .prose :global(ul),
  .prose :global(ol) {
    margin: 0 0 var(--space-4);
    padding-left: var(--space-6);
    color: var(--color-fg);
  }

  .prose :global(li) {
    font-size: var(--font-size-base);
    line-height: var(--line-height-loose);
    margin-bottom: var(--space-1);
  }

  .prose :global(table) {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--font-size-sm);
    margin: var(--space-5) 0;
  }

  .prose :global(th) {
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    padding: var(--space-2) var(--space-3);
    text-align: left;
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
  }

  .prose :global(td) {
    border: 1px solid var(--color-border);
    padding: var(--space-2) var(--space-3);
    color: var(--color-fg);
    vertical-align: top;
  }

  .prose :global(hr) {
    border: none;
    border-top: 1px solid var(--color-border);
    margin: var(--space-6) 0;
  }

  .prose :global(a) {
    color: var(--color-accent);
    text-decoration: none;
    border-bottom: 1px solid transparent;
    transition: border-color var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .prose :global(a:hover),
  .prose :global(a:focus-visible) {
    border-bottom-color: currentColor;
  }

  .prose :global(a.cross-ref) {
    font-family: var(--font-mono);
    font-size: 0.875em;
    color: var(--color-accent-muted);
  }

  /* Paper footer */
  .paper-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: var(--space-3);
    padding-top: var(--space-6);
    border-top: 1px solid var(--color-border);
    margin-top: var(--space-7);
  }

  .footer-link {
    font-size: var(--font-size-sm);
    color: var(--color-accent);
    text-decoration: none;
    border-bottom: 1px solid transparent;
  }

  .footer-link:hover {
    border-bottom-color: currentColor;
  }

  .footer-downstream {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    display: flex;
    align-items: center;
    gap: var(--space-1);
  }

  /* Error state */
  .error-state {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    align-items: flex-start;
  }

  /* Scope bar breadcrumb */
  .breadcrumb-root {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    text-decoration: none;
    transition: color var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .breadcrumb-root:hover,
  .breadcrumb-root:focus-visible {
    color: var(--color-fg);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .breadcrumb-sep {
    font-size: var(--font-size-xs);
    color: var(--color-border-strong);
  }

  .breadcrumb-id {
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
  }

  .breadcrumb-section {
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-fg-muted);
  }

  @media (prefers-reduced-motion: reduce) {
    .toc-link,
    .breadcrumb-root {
      transition: none;
    }
  }
</style>
