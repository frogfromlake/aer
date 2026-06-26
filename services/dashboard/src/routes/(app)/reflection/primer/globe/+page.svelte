<script lang="ts">
  // Surface III — "How to read the globe" primer (Phase 109).
  // Linked from Surface I's scope bar (Phase 110) and from the Reflection landing.
  // Content fetched from the Content Catalog API (primer entity type);
  // falls back to static embedded prose when the API is unavailable.
  import { createQuery } from '@tanstack/svelte-query';
  import { ScopeBar } from '$lib/components/chrome';
  import ReflectionBackLink from '$lib/components/reflection/ReflectionBackLink.svelte';
  import ProgressiveSemantics from '$lib/components/ProgressiveSemantics.svelte';
  import {
    contentQuery,
    type ContentResponseDto,
    type FetchContext,
    type QueryOutcome
  } from '$lib/api/queries';
  import { locale } from '$lib/state/locale.svelte';
  import { m } from '$lib/paraglide/messages.js';

  const ctx: FetchContext = { baseUrl: '/api/v1' };

  const contentQ = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const o = contentQuery(ctx, 'primer', 'globe_primer', locale());
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  const contentRecord = $derived<ContentResponseDto | null>(
    contentQ.data?.kind === 'success' ? contentQ.data.data : null
  );

  // Fallback static sections when the API is unavailable. Derived so the prose
  // re-renders on a language switch.
  const STATIC_SECTIONS = $derived<Array<{ heading: string; body: string }>>([
    {
      heading: m.reflection_primer_static_probe_heading(),
      body: m.reflection_primer_static_probe_body()
    },
    {
      heading: m.reflection_primer_static_functions_heading(),
      body: m.reflection_primer_static_functions_body()
    },
    {
      heading: m.reflection_primer_static_shows_heading(),
      body: m.reflection_primer_static_shows_body()
    },
    {
      heading: m.reflection_primer_static_descend_heading(),
      body: m.reflection_primer_static_descend_body()
    }
  ]);
</script>

<svelte:head>
  <title>{m.reflection_primer_head_title()}</title>
</svelte:head>

<ScopeBar label={m.reflection_primer_scopebar_label()}>
  <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
  <a href="/reflection" class="breadcrumb-root">{m.reflection_primer_breadcrumb_root()}</a>
  <span class="breadcrumb-sep" aria-hidden="true">›</span>
  <span class="breadcrumb-current" aria-current="page"
    >{m.reflection_primer_breadcrumb_current()}</span
  >
</ScopeBar>

<main class="primer-main" id="main-primer-globe">
  <!-- Always-reachable back-to-Reflection control, same as the other Reflection
       pages (open-questions / WP). Sticky pill in the top-left gutter. -->
  <ReflectionBackLink />
  <div class="primer-inner">
    <header class="primer-header">
      <p class="primer-eyebrow">{m.reflection_primer_eyebrow()}</p>
      <h1 class="primer-title">{m.reflection_primer_title()}</h1>
      <p class="primer-sub">
        {m.reflection_primer_sub()}
      </p>
    </header>

    {#if contentQ.isPending}
      <p class="state-msg" aria-busy="true">{m.reflection_primer_loading()}</p>
    {:else if contentRecord}
      <!-- Content Catalog response -->
      <div class="primer-body">
        <ProgressiveSemantics registers={contentRecord.registers} emphasis="semantic" />
      </div>
    {:else}
      <!-- Static fallback prose -->
      <div class="primer-body">
        {#each STATIC_SECTIONS as sec (sec.heading)}
          <section class="primer-section" aria-labelledby={sec.heading.replace(/\s+/g, '-')}>
            <h2 class="primer-section-title" id={sec.heading.replace(/\s+/g, '-')}>
              {sec.heading}
            </h2>
            <div class="primer-section-body">
              <!-- eslint-disable-next-line svelte/no-at-html-tags -->
              {@html sec.body}
            </div>
          </section>
        {/each}
      </div>
    {/if}

    <!-- Cross-links -->
    <nav class="primer-nav" aria-label={m.reflection_primer_further_reading_aria()}>
      <h2 class="primer-nav-title">{m.reflection_primer_further_reading_heading()}</h2>
      <ul class="primer-nav-list" role="list">
        <li>
          <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
          <a href="/reflection/wp/wp-001" class="primer-nav-link">
            <span class="nav-link-id">WP-001</span>
            {m.reflection_primer_further_reading_wp001()}
          </a>
        </li>
        <li>
          <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
          <a href="/reflection/wp/wp-003" class="primer-nav-link">
            <span class="nav-link-id">WP-003</span>
            {m.reflection_primer_further_reading_wp003()}
          </a>
        </li>
        <li>
          <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
          <a href="/reflection/wp/wp-006?section=5" class="primer-nav-link">
            <span class="nav-link-id">WP-006 §5</span>
            {m.reflection_primer_further_reading_wp006()}
          </a>
        </li>
        <li>
          <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
          <a href="/reflection/open-questions" class="primer-nav-link">
            {m.reflection_primer_further_reading_open_questions()}
          </a>
        </li>
      </ul>
    </nav>
  </div>
</main>

<style>
  .primer-main {
    position: fixed;
    inset: 0;
    left: var(--rail-width);
    top: var(--scope-bar-height);
    right: var(--tray-right-edge, var(--tray-closed-width));
    overflow-y: auto;
    background: color-mix(in srgb, var(--color-bg) 72%, transparent);
    backdrop-filter: blur(3px);
    -webkit-backdrop-filter: blur(3px);
  }

  .primer-inner {
    max-width: 66ch;
    margin: 0 auto;
    padding: var(--space-7) var(--space-6) var(--space-9);
    display: flex;
    flex-direction: column;
    gap: var(--space-7);
  }

  .primer-eyebrow {
    font-size: var(--font-size-xs);
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    margin: 0 0 var(--space-2);
    font-family: var(--font-mono);
  }

  .primer-title {
    font-size: var(--font-size-3xl);
    font-weight: var(--font-weight-semibold);
    letter-spacing: var(--letter-spacing-tight);
    color: var(--color-fg);
    margin: 0 0 var(--space-3);
    line-height: var(--line-height-tight);
  }

  .primer-sub {
    font-size: var(--font-size-md);
    line-height: var(--line-height-loose);
    color: var(--color-fg-muted);
    margin: 0;
  }

  .state-msg {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  .primer-body {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }

  .primer-section {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .primer-section-title {
    font-size: var(--font-size-lg);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    margin: 0;
    line-height: var(--line-height-tight);
  }

  .primer-section-body {
    font-size: var(--font-size-base);
    line-height: var(--line-height-loose);
    color: var(--color-fg);
  }

  .primer-section-body :global(strong) {
    font-weight: var(--font-weight-semibold);
  }

  .primer-section-body :global(ul) {
    padding-left: var(--space-6);
    margin: var(--space-3) 0;
  }

  .primer-section-body :global(li) {
    line-height: var(--line-height-loose);
    margin-bottom: var(--space-1);
  }

  /* Further reading nav */
  .primer-nav {
    border-top: 1px solid var(--color-border);
    padding-top: var(--space-5);
  }

  .primer-nav-title {
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-semibold);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-muted);
    margin: 0 0 var(--space-3);
  }

  .primer-nav-list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .primer-nav-link {
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
    font-size: var(--font-size-sm);
    color: var(--color-accent);
    text-decoration: none;
  }

  .primer-nav-link:hover,
  .primer-nav-link:focus-visible {
    text-decoration: underline;
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .nav-link-id {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    font-weight: var(--font-weight-semibold);
    flex-shrink: 0;
  }

  /* ScopeBar */
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

  .breadcrumb-current {
    font-size: var(--font-size-xs);
    color: var(--color-fg);
    font-weight: var(--font-weight-medium);
  }

  @media (prefers-reduced-motion: reduce) {
    .breadcrumb-root {
      transition: none;
    }
  }
</style>
