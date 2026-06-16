<script lang="ts">
  // WpBreadcrumb — Phase 141 (extracted from wp/[id]/+page.svelte). The ScopeBar
  // breadcrumb content for the Working-Paper reader: the Reflection root link,
  // the paper id, the active §section, and the Back-to-Workbench link shown when
  // the reader arrived from a Pillar cell (Phase 113c / 122h referrer params).
  import { page } from '$app/state';
  import { buildBackToWorkbenchHref } from '$lib/reflection/wp-page-internals';

  interface Props {
    paperId: string | null;
    sectionParam: string | null;
  }
  let { paperId, sectionParam }: Props = $props();

  const fromReferrer = $derived(
    page.url.searchParams.get('from') === 'workbench' ||
      page.url.searchParams.get('from') === 'lane'
  );
  const backToLaneHref = $derived(
    buildBackToWorkbenchHref({
      probe: fromReferrer ? page.url.searchParams.get('probe') : null,
      fn: fromReferrer ? page.url.searchParams.get('fn') : null,
      pillar: fromReferrer ? page.url.searchParams.get('pillar') : null
    })
  );
</script>

<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
<a href="/reflection" class="breadcrumb-root" aria-label="Back to Reflection surface">
  Reflection
</a>
<span class="breadcrumb-sep" aria-hidden="true">›</span>
<span class="breadcrumb-id" aria-current="page">
  {paperId ? paperId.toUpperCase() : '…'}
</span>
{#if sectionParam}
  <span class="breadcrumb-sep" aria-hidden="true">›</span>
  <span class="breadcrumb-section">§{sectionParam}</span>
{/if}
{#if backToLaneHref}
  <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
  <a class="back-to-lane" href={backToLaneHref} aria-label="Back to Workbench">
    ← Back to Workbench
  </a>
{/if}

<style>
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

  .back-to-lane {
    margin-left: auto;
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-accent);
    text-decoration: none;
    border: 1px solid var(--color-accent-muted);
    border-radius: var(--radius-sm);
    padding: 2px var(--space-2);
  }
  .back-to-lane:hover,
  .back-to-lane:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-accent);
    background: rgba(82, 131, 184, 0.12);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  @media (prefers-reduced-motion: reduce) {
    .breadcrumb-root {
      transition: none;
    }
  }
</style>
