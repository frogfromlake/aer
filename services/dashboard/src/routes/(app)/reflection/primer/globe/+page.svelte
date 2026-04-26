<script lang="ts">
  // Surface III — "How to read the globe" primer (Phase 109).
  // Linked from Surface I's scope bar (Phase 110) and from the Reflection landing.
  // Content fetched from the Content Catalog API (primer entity type);
  // falls back to static embedded prose when the API is unavailable.
  import { createQuery } from '@tanstack/svelte-query';
  import { ScopeBar } from '$lib/components/chrome';
  import ProgressiveSemantics from '$lib/components/ProgressiveSemantics.svelte';
  import {
    contentQuery,
    type ContentResponseDto,
    type FetchContext,
    type QueryOutcome
  } from '$lib/api/queries';

  const ctx: FetchContext = { baseUrl: '/api/v1' };

  const contentQ = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const o = contentQuery(ctx, 'primer', 'globe_primer', 'en');
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  const contentRecord = $derived<ContentResponseDto | null>(
    contentQ.data?.kind === 'success' ? contentQ.data.data : null
  );

  // Fallback static sections when the API is unavailable
  const STATIC_SECTIONS: Array<{ heading: string; body: string }> = [
    {
      heading: 'What is a probe?',
      body: `AĒR observes global discourse through <em>probes</em> — strategic observation points within the global information space. Each probe is a grouping of sources that share a cultural and discursive scope. Probe 0, the current proof-of-concept, covers German institutional public discourse through two sources: Tagesschau (public broadcaster RSS) and Bundesregierung (federal government press releases).

A probe is not a country. It is a constellation of sources chosen for the discursive <em>functions</em> they serve in their society — not for their nationality or institutional label.`
    },
    {
      heading: 'The four discourse functions',
      body: `Every source in a probe is classified by the discursive function it serves (following WP-001):
      <ul>
        <li><strong>Epistemic Authority</strong> — sources that define what is considered true, credible, and legitimate knowledge in a given society.</li>
        <li><strong>Power Legitimation</strong> — sources that articulate and justify the exercise of political, economic, or social power.</li>
        <li><strong>Cohesion &amp; Identity</strong> — sources that construct, reinforce, or contest collective identities and social bonds.</li>
        <li><strong>Subversion &amp; Friction</strong> — sources that challenge dominant narratives, amplify marginalized voices, or contest hegemonic frames.</li>
      </ul>
      Probe 0's two sources collectively cover Epistemic Authority (Tagesschau) and Power Legitimation (Bundesregierung).`
    },
    {
      heading: 'What the globe shows — and what it does not',
      body: `On the Atmosphere surface, you see luminous glyphs where active probes are monitoring. The day/night terminator marks the current UTC boundary. Probe glyph brightness encodes recent publication activity.

<strong>What AĒR does not show</strong> is as important as what it does. Large parts of the globe are currently unmonitored — no probe, no data. The Negative Space overlay (activatable from the left rail) makes these absences visible rather than naturalizing them. AĒR's coverage is not a map of the world's discourse. It is a map of where AĒR has chosen to look, so far.`
    },
    {
      heading: 'Descending from the globe',
      body: `The globe is the landing overview, not the primary working surface. To do scientific work, descend to Surface II (Function Lanes) via the side rail or by clicking a probe glyph. There you will find time-series data, entity co-occurrence networks, and the full view-mode matrix.

Surface III (here) is where methodology becomes legible — every metric's provenance, every probe's dossier, and the six Working Papers that define AĒR's scientific foundations.`
    }
  ];
</script>

<svelte:head>
  <title>AĒR — How to Read the Globe</title>
</svelte:head>

<ScopeBar label="Reflection — Globe primer navigation">
  <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
  <a href="/reflection" class="breadcrumb-root">Reflection</a>
  <span class="breadcrumb-sep" aria-hidden="true">›</span>
  <span class="breadcrumb-current" aria-current="page">How to Read the Globe</span>
</ScopeBar>

<main class="primer-main" id="main-primer-globe">
  <div class="primer-inner">
    <header class="primer-header">
      <p class="primer-eyebrow">Primer</p>
      <h1 class="primer-title">How to Read the Globe</h1>
      <p class="primer-sub">
        An introduction to the Atmosphere surface — what probes are, what the globe shows, and
        critically, what it does not show.
      </p>
    </header>

    {#if contentQ.isPending}
      <p class="state-msg" aria-busy="true">Loading primer content…</p>
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
    <nav class="primer-nav" aria-label="Further reading">
      <h2 class="primer-nav-title">Further reading</h2>
      <ul class="primer-nav-list" role="list">
        <li>
          <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
          <a href="/reflection/wp/wp-001" class="primer-nav-link">
            <span class="nav-link-id">WP-001</span>
            Toward a Culturally Agnostic Probe Catalog
          </a>
        </li>
        <li>
          <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
          <a href="/reflection/wp/wp-003" class="primer-nav-link">
            <span class="nav-link-id">WP-003</span>
            Platform Bias, Algorithmic Amplification, and the Detection of Non-Human Actors
          </a>
        </li>
        <li>
          <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
          <a href="/reflection/wp/wp-006?section=5" class="primer-nav-link">
            <span class="nav-link-id">WP-006 §5</span>
            The Ethics of Making Discourse Visible
          </a>
        </li>
        <li>
          <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
          <a href="/reflection/open-questions" class="primer-nav-link">
            Open Research Questions →
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
    background: var(--color-bg);
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
