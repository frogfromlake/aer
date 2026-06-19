<script lang="ts">
  // WpTableOfContents — Phase 141 (extracted from wp/[id]/+page.svelte). The
  // sticky TOC sidebar: the scope-boundary "absence margin" annotation (Design
  // Brief §5.4) + the numbered/appendix section links. Pure presentation; the
  // parent gates rendering on `mainSections.length > 0`.
  import type { PaperSection } from '$lib/reflection/md';
  import { m } from '$lib/paraglide/messages.js';

  interface Props {
    mainSections: PaperSection[];
    appendixSections: PaperSection[];
  }
  let { mainSections, appendixSections }: Props = $props();
</script>

<nav class="toc" style="margin-right: -4rem;" aria-label={m.reflection_wp_toc_aria_label()}>
  <!-- Surface III — scope boundary annotation (Phase 112), shown by default now
       that the Negative-Space toggle is retired. Absence-prose scrolls alongside
       the paper body per Design Brief §5.4. -->
  <aside class="absence-margin" aria-label={m.reflection_wp_toc_scope_boundary_notes_aria()}>
    <p class="absence-margin-heading">
      <span aria-hidden="true">∅</span>
      {m.reflection_wp_toc_scope_boundary_heading()}
    </p>
    <p class="absence-margin-text">
      {m.reflection_wp_toc_scope_boundary_text()}
    </p>
    <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
    <a class="absence-margin-ref" href="/reflection/wp/wp-001?section=5.3">WP-001 §5.3</a>
  </aside>
  <hr class="absence-divider" aria-hidden="true" />
  <p class="toc-heading">{m.reflection_wp_toc_heading()}</p>
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
            <span class="toc-num">{m.reflection_wp_toc_appendix_prefix()} {s.number}</span>
            {s.title}
          </a>
        </li>
      {/each}
    {/if}
  </ol>
</nav>

<style>
  /* Table of contents. The Negative-Space TOC accent (formerly `.neg-space .toc`)
     is folded into the base rule — the `.wp-layout` always carries `neg-space`
     since the Phase-112 toggle was retired, so the accent always applied. */
  .toc {
    border-right: 1px solid rgba(82, 131, 184, 0.4);
    overflow-y: auto;
    padding: var(--space-5) var(--space-3);
    /* Same glassy treatment as the ScopeBar (top bar). */
    background: var(--color-bg-overlay);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
  }

  @media (max-width: 900px) {
    .toc {
      display: none;
    }
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

  @media (prefers-reduced-motion: reduce) {
    .toc-link {
      transition: none;
    }
  }
</style>
