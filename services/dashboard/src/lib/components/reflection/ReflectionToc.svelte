<script lang="ts">
  // ReflectionToc — a generic in-page anchor sidebar (Phase 127), the same sticky
  // table-of-contents pattern the Working Paper reader uses (WpTableOfContents),
  // generalised for every long Reflection surface (open questions, the probe and
  // metric aggregates). Items are {id, label} pairs anchoring to `#id` on the page.
  import { m } from '$lib/paraglide/messages.js';

  interface TocItem {
    id: string;
    label: string;
    num?: string;
  }
  interface Props {
    items: TocItem[];
    heading?: string;
  }
  let { items, heading = m.reflection_toc_heading() }: Props = $props();
</script>

<nav class="toc" aria-label={heading}>
  <p class="toc-heading">{heading}</p>
  <ol class="toc-list">
    {#each items as item (item.id)}
      <li>
        <a href="#{item.id}" class="toc-link">
          {#if item.num}<span class="toc-num">{item.num}</span>{/if}
          {item.label}
        </a>
      </li>
    {/each}
  </ol>
</nav>

<style>
  /* Mirrors WpTableOfContents — the Negative-Space accent the WP layout always
     carries, applied uniformly across the Reflection surface. */
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
