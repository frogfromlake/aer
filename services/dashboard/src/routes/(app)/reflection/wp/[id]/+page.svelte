<script lang="ts">
  // Surface III — Working Paper reader (Phase 109).
  // Fetches the paper markdown from /content/papers/{id}.md, renders it with the
  // minimal GFM renderer, and presents it as long-form Distill-style prose.
  //
  // Phase 141 — this file is now the page shell: it owns the data → derived
  // wiring, the ?section= scroll-to-section onMount, and the layout grid; the
  // three regions live in children (WpBreadcrumb / WpTableOfContents /
  // WpPaperBody) and the pure transforms in `wp-page-internals.ts`.
  import { page } from '$app/state';
  import { onMount } from 'svelte';
  import { ScopeBar } from '$lib/components/chrome';
  import WpBreadcrumb from '$lib/components/reflection/WpBreadcrumb.svelte';
  import WpTableOfContents from '$lib/components/reflection/WpTableOfContents.svelte';
  import WpPaperBody from '$lib/components/reflection/WpPaperBody.svelte';
  import { splitSections, scrollTargetIds } from '$lib/reflection/wp-page-internals';
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

  // Split once for the TOC (numbered/appendix) — the body renders all in order.
  const toc = $derived(splitSections(sections));

  // Scroll to the requested section after mount
  onMount(() => {
    if (!sectionParam) return;
    const tryScroll = () => {
      for (const id of scrollTargetIds(sectionParam)) {
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
</script>

<svelte:head>
  <title>AĒR — {title}</title>
</svelte:head>

<ScopeBar label="Reflection — Working Paper navigation">
  <WpBreadcrumb paperId={meta?.id ?? null} {sectionParam} />
</ScopeBar>

<div class="wp-layout neg-space" id="main-reflection-wp">
  <!-- Table of contents / absence margin (sticky sidebar on wide screens) -->
  {#if toc.main.length > 0}
    <WpTableOfContents mainSections={toc.main} appendixSections={toc.appendix} />
  {/if}

  <!-- Paper body -->
  <WpPaperBody {paper} {title} />
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
    background: color-mix(in srgb, var(--color-bg) 72%, transparent);
    backdrop-filter: blur(3px);
    -webkit-backdrop-filter: blur(3px);
  }

  @media (max-width: 900px) {
    .wp-layout {
      grid-template-columns: 1fr;
    }
  }
</style>
