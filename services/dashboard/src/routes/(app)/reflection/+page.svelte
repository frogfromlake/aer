<script lang="ts">
  /* eslint-disable svelte/no-navigation-without-resolve -- internal back-to-globe navigation */
  // Surface III — Reflection landing (Phase 109).
  // Presents the WP index, primer entry, and open-questions entry.
  // Replaces the Phase 105 stub.
  import { goto } from '$app/navigation';
  import { ScopeBar } from '$lib/components/chrome';
  import { getAllPapers } from '$lib/reflection/papers';
  import { OPEN_QUESTIONS } from '$lib/reflection/open-questions';
  import { m } from '$lib/paraglide/messages.js';
  import {
    paperShortTitle,
    paperAbstract,
    paperStatus
  } from '$lib/components/reflection/paper-display';

  const papers = getAllPapers();
  const questionCount = OPEN_QUESTIONS.length;
</script>

<svelte:head>
  <title>{m.reflection_head_title()}</title>
</svelte:head>

<!-- Phase 135 — the Reflection landing reads as a glassy surface over the
     layout's persistent globe, matching the global overlays. -->
<ScopeBar label={m.reflection_landing_scopebar_label()}>
  <span class="surface-label">{m.reflection_landing_surface_label()}</span>
  <span class="surface-sub">{m.reflection_landing_surface_sub()}</span>
</ScopeBar>

<button
  type="button"
  class="reflection-close"
  onclick={() => goto('/')}
  aria-label={m.reflection_landing_close()}
  title={m.reflection_landing_close_title()}
>
  ×
</button>

<main class="reflection-landing" id="main-reflection">
  <div class="landing-inner">
    <!-- Surface identity -->
    <header class="landing-header">
      <p class="surface-eyebrow">{m.reflection_landing_eyebrow()}</p>
      <h1 class="landing-title">{m.reflection_landing_title()}</h1>
      <p class="landing-abstract">
        {m.reflection_landing_abstract()}
      </p>
    </header>

    <!-- Entry points grid -->
    <div class="entry-grid">
      <!-- Open Research Questions -->
      <section class="entry-card" aria-labelledby="oq-heading">
        <h2 id="oq-heading" class="entry-title">{m.reflection_landing_oq_heading()}</h2>
        <p class="entry-body">
          {m.reflection_landing_oq_body({ count: questionCount })}
        </p>
        <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
        <a href="/reflection/open-questions" class="entry-link">
          {m.reflection_landing_oq_link()}
        </a>
      </section>

      <!-- Globe primer -->
      <section class="entry-card" aria-labelledby="primer-heading">
        <h2 id="primer-heading" class="entry-title">{m.reflection_landing_primer_heading()}</h2>
        <p class="entry-body">
          {m.reflection_landing_primer_body_pre()}
          <em>{m.reflection_landing_primer_body_emphasis()}</em>
          {m.reflection_landing_primer_body_post()}
        </p>
        <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
        <a href="/reflection/primer/globe" class="entry-link">
          {m.reflection_landing_primer_link()}
        </a>
      </section>
    </div>

    <!-- Working Papers index -->
    <section class="section" aria-labelledby="wps-heading">
      <h2 id="wps-heading" class="section-title">{m.reflection_landing_wps_heading()}</h2>
      <p class="section-sub">
        {m.reflection_landing_wps_sub()}
      </p>
      <ul class="paper-list" role="list">
        {#each papers as p (p.id)}
          {@const status = paperStatus(p.id)}
          <li>
            <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
            <a href="/reflection/wp/{p.id}" class="paper-card">
              <div class="paper-card-head">
                <span class="paper-id">{p.id.toUpperCase()}</span>
                <span class="paper-status">{status.split('—')[0]?.trim() ?? status}</span>
              </div>
              <p class="paper-short">{paperShortTitle(p.id)}</p>
              <p class="paper-abstract">{paperAbstract(p.id)}</p>
            </a>
          </li>
        {/each}
      </ul>
    </section>

    <!-- Catalogue entry points — one tile each into the probe + metric
         aggregates, where every dossier / provenance record is read inline. -->
    <div class="entry-grid">
      <!-- Probe dossiers -->
      <section class="entry-card" aria-labelledby="probes-heading">
        <h2 id="probes-heading" class="entry-title">{m.reflection_landing_probes_heading()}</h2>
        <p class="entry-body">{m.reflection_landing_probes_sub()}</p>
        <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
        <a href="/reflection/probes" class="entry-link">{m.reflection_landing_probes_link()}</a>
      </section>

      <!-- Metric provenance -->
      <section class="entry-card" aria-labelledby="metrics-heading">
        <h2 id="metrics-heading" class="entry-title">{m.reflection_landing_metrics_heading()}</h2>
        <p class="entry-body">{m.reflection_landing_metrics_sub()}</p>
        <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
        <a href="/reflection/metrics" class="entry-link">{m.reflection_landing_metrics_link()}</a>
      </section>
    </div>

    <!-- Footer note -->
    <p class="landing-footer-note">
      {m.reflection_landing_footer_note()}
    </p>
  </div>
</main>

<style>
  .reflection-landing {
    position: fixed;
    inset: 0;
    left: var(--rail-width);
    top: var(--scope-bar-height);
    right: var(--tray-right-edge, var(--tray-closed-width));
    z-index: 1;
    overflow-y: auto;
    /* Glassy dim over the layout's persistent globe — same feel as the overlays. */
    background: color-mix(in srgb, var(--color-bg) 72%, transparent);
    backdrop-filter: blur(3px);
    -webkit-backdrop-filter: blur(3px);
  }

  .reflection-close {
    position: fixed;
    top: calc(var(--scope-bar-height) + var(--space-3));
    right: calc(var(--tray-right-edge, var(--tray-closed-width)) + var(--space-4));
    z-index: 5;
    background: transparent;
    border: none;
    color: var(--color-fg-muted);
    font-size: var(--font-size-2xl);
    line-height: 1;
    cursor: pointer;
    padding: 0 var(--space-2);
  }
  .reflection-close:hover,
  .reflection-close:focus-visible {
    color: var(--color-fg);
    outline: none;
  }

  .landing-inner {
    max-width: 72ch;
    margin: 0 auto;
    padding: var(--space-7) var(--space-6) var(--space-9);
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }

  /* Header */
  .surface-eyebrow {
    font-size: var(--font-size-xs);
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    margin: 0 0 var(--space-2);
    font-family: var(--font-mono);
  }

  .landing-title {
    font-size: var(--font-size-3xl);
    font-weight: var(--font-weight-semibold);
    letter-spacing: var(--letter-spacing-tight);
    color: var(--color-fg);
    margin: 0 0 var(--space-4);
    line-height: var(--line-height-tight);
  }

  .landing-abstract {
    font-size: var(--font-size-md);
    line-height: var(--line-height-loose);
    color: var(--color-fg-muted);
    margin: 0;
  }

  /* Section */
  .section {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .section-title {
    font-size: var(--font-size-lg);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    margin: 0;
  }

  .section-sub {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
    margin: 0;
  }

  /* Paper list */
  .paper-list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .paper-card {
    display: block;
    padding: var(--space-4);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    text-decoration: none;
    transition: all var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .paper-card:hover,
  .paper-card:focus-visible {
    border-color: var(--color-accent-muted);
    background: var(--color-surface-hover);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .paper-card-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    margin-bottom: var(--space-2);
  }

  .paper-id {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    font-weight: var(--font-weight-semibold);
    color: var(--color-accent);
    letter-spacing: 0.05em;
  }

  .paper-status {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    font-style: italic;
  }

  .paper-short {
    font-size: var(--font-size-base);
    font-weight: var(--font-weight-medium);
    color: var(--color-fg);
    margin: 0 0 var(--space-2);
  }

  .paper-abstract {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
    margin: 0;
  }

  /* Entry grid */
  .entry-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-4);
  }

  @media (max-width: 640px) {
    .entry-grid {
      grid-template-columns: 1fr;
    }
  }

  .entry-card {
    padding: var(--space-5);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .entry-title {
    font-size: var(--font-size-base);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    margin: 0;
  }

  .entry-body {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
    margin: 0;
    flex: 1;
  }

  .entry-link {
    font-size: var(--font-size-sm);
    color: var(--color-accent);
    text-decoration: none;
    border-bottom: 1px solid transparent;
    align-self: flex-start;
  }

  .entry-link:hover,
  .entry-link:focus-visible {
    border-bottom-color: currentColor;
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  /* Footer note */
  .landing-footer-note {
    font-size: var(--font-size-sm);
    color: var(--color-fg-subtle);
    line-height: var(--line-height-loose);
    border-top: 1px solid var(--color-border);
    padding-top: var(--space-5);
    margin: 0;
  }

  /* ScopeBar content */
  .surface-label {
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-medium);
    color: var(--color-fg);
  }

  .surface-sub {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    font-family: var(--font-mono);
  }

  @media (prefers-reduced-motion: reduce) {
    .paper-card {
      transition: none;
    }
  }
</style>
