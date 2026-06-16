<script lang="ts">
  // WpPaperBody — Phase 141 (extracted from wp/[id]/+page.svelte). The paper
  // article: header (series/title/meta), the rendered markdown sections with
  // their inline interactive cells, and the footer (open-questions + downstream
  // links). Owns the long-form prose typography (`.prose :global(*)`). Derives
  // meta/rendered/sections from the single `paper` prop so the parent stays thin.
  import InlineChart from '$lib/components/reflection/InlineChart.svelte';
  import type { PaperMeta } from '$lib/reflection/papers';
  import type { ParsedPaper } from '$lib/reflection/md';

  interface Props {
    paper: { meta: PaperMeta; rendered: ParsedPaper | null } | null;
    title: string;
  }
  let { paper, title }: Props = $props();

  const meta = $derived(paper?.meta ?? null);
  const rendered = $derived(paper?.rendered ?? null);
  const sections = $derived(rendered?.sections ?? []);
</script>

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

<style>
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
</style>
