<script lang="ts">
  // Surface III — Reflection landing (Phase 109).
  // Presents the WP index, primer entry, and open-questions entry.
  // Replaces the Phase 105 stub.
  import { ScopeBar } from '$lib/components/chrome';
  import { getAllPapers } from '$lib/reflection/papers';
  import { OPEN_QUESTIONS } from '$lib/reflection/open-questions';

  const papers = getAllPapers();
  const questionCount = OPEN_QUESTIONS.length;
</script>

<svelte:head>
  <title>AĒR — Reflection</title>
</svelte:head>

<ScopeBar label="Reflection surface navigation">
  <span class="surface-label">Reflection</span>
  <span class="surface-sub">Working Papers · Primers · Open Questions</span>
</ScopeBar>

<main class="reflection-landing" id="main-reflection">
  <div class="landing-inner">
    <!-- Surface identity -->
    <header class="landing-header">
      <p class="surface-eyebrow">Surface III</p>
      <h1 class="landing-title">Reflection</h1>
      <p class="landing-abstract">
        The primary methodological surface. Where every metric's provenance, every probe's dossier,
        every known limitation, and every Working Paper lives as long-form, typographically
        disciplined content. A dashboard without it is a surveillance tool; with it peripheral, a
        research tool with an alibi.
      </p>
    </header>

    <!-- Working Papers index -->
    <section class="section" aria-labelledby="wps-heading">
      <h2 id="wps-heading" class="section-title">Working Papers</h2>
      <p class="section-sub">
        Six papers defining the methodological foundations of AĒR — probe selection, metric
        validity, platform bias, cross-cultural comparability, temporal granularity, and the
        observer effect.
      </p>
      <ul class="paper-list" role="list">
        {#each papers as p (p.id)}
          <li>
            <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
            <a href="/reflection/wp/{p.id}" class="paper-card">
              <div class="paper-card-head">
                <span class="paper-id">{p.id.toUpperCase()}</span>
                <span class="paper-status">{p.status.split('—')[0]?.trim() ?? p.status}</span>
              </div>
              <p class="paper-short">{p.shortTitle}</p>
              <p class="paper-abstract">{p.abstract}</p>
            </a>
          </li>
        {/each}
      </ul>
    </section>

    <!-- Entry points grid -->
    <div class="entry-grid">
      <!-- Open Research Questions -->
      <section class="entry-card" aria-labelledby="oq-heading">
        <h2 id="oq-heading" class="entry-title">Open Research Questions</h2>
        <p class="entry-body">
          {questionCount} questions requiring interdisciplinary collaboration, gathered from the WP §7
          and §8 sections. The questions that define where AĒR needs expertise beyond software engineering.
        </p>
        <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
        <a href="/reflection/open-questions" class="entry-link"> Browse all questions → </a>
      </section>

      <!-- Globe primer -->
      <section class="entry-card" aria-labelledby="primer-heading">
        <h2 id="primer-heading" class="entry-title">How to Read the Globe</h2>
        <p class="entry-body">
          A short introduction to Surface I — what probe glyphs represent, what the day/night
          terminator means, and — importantly — what AĒR does
          <em>not</em> show (the Negative Space framing).
        </p>
        <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
        <a href="/reflection/primer/globe" class="entry-link"> Read the primer → </a>
      </section>
    </div>

    <!-- Footer note -->
    <p class="landing-footer-note">
      Reflection is reachable from every metric badge, every refusal surface, and every "Read the
      full Working Paper" link in the Methodology Tray.
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
    overflow-y: auto;
    background: var(--color-bg);
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
