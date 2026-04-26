<script lang="ts">
  // Surface III — Open Research Questions hub (Phase 109).
  // Renders all questions from OPEN_QUESTIONS grouped by source WP.
  import { ScopeBar } from '$lib/components/chrome';
  import { questionsByWp, OPEN_QUESTIONS } from '$lib/reflection/open-questions';
  import { getAllPapers } from '$lib/reflection/papers';

  const grouped = questionsByWp();
  const papers = getAllPapers();
  const total = OPEN_QUESTIONS.length;
</script>

<svelte:head>
  <title>AĒR — Open Research Questions</title>
</svelte:head>

<ScopeBar label="Reflection — Open Research Questions navigation">
  <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
  <a href="/reflection" class="breadcrumb-root">Reflection</a>
  <span class="breadcrumb-sep" aria-hidden="true">›</span>
  <span class="breadcrumb-current" aria-current="page">Open Research Questions</span>
</ScopeBar>

<main class="oq-main" id="main-open-questions">
  <div class="oq-inner">
    <header class="oq-header">
      <h1 class="oq-title">Open Research Questions</h1>
      <p class="oq-abstract">
        {total} questions gathered from the six AĒR Working Papers (§7 and §8 sections of each), each
        identifying a gap that requires expertise beyond software engineering. Grouped by paper and discipline.
      </p>
    </header>

    {#each papers as paper (paper.id)}
      {@const questions = grouped.get(paper.id) ?? []}
      {#if questions.length > 0}
        <section class="wp-group" aria-labelledby="group-{paper.id}">
          <div class="wp-group-header">
            <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
            <a href="/reflection/wp/{paper.id}" class="wp-group-id">
              {paper.id.toUpperCase()}
            </a>
            <h2 id="group-{paper.id}" class="wp-group-title">{paper.shortTitle}</h2>
            <span class="wp-group-count">{questions.length} questions</span>
          </div>

          <ul class="oq-list" role="list">
            {#each questions as q (q.id)}
              <li class="oq-item">
                <div class="oq-item-head">
                  <span class="oq-id">{q.id}</span>
                  <span class="oq-discipline">{q.disciplinaryScope}</span>
                </div>
                <p class="oq-label">{q.shortLabel}</p>
                <p class="oq-question">{q.question}</p>
                {#if q.deliverable}
                  <p class="oq-deliverable">
                    <span class="deliverable-label">Deliverable:</span>
                    {q.deliverable}
                  </p>
                {/if}
                {#if q.pipelineHook}
                  <p class="oq-hook">
                    <span class="hook-label">AĒR pipeline hook:</span>
                    {q.pipelineHook}
                  </p>
                {/if}
              </li>
            {/each}
          </ul>
        </section>
      {/if}
    {/each}

    <footer class="oq-footer">
      <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
      <a href="/reflection" class="back-link">← Back to Reflection</a>
    </footer>
  </div>
</main>

<style>
  .oq-main {
    position: fixed;
    inset: 0;
    left: var(--rail-width);
    top: var(--scope-bar-height);
    right: var(--tray-right-edge, var(--tray-closed-width));
    overflow-y: auto;
    background: var(--color-bg);
  }

  .oq-inner {
    max-width: 72ch;
    margin: 0 auto;
    padding: var(--space-7) var(--space-6) var(--space-9);
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }

  .oq-header {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .oq-title {
    font-size: var(--font-size-3xl);
    font-weight: var(--font-weight-semibold);
    letter-spacing: var(--letter-spacing-tight);
    color: var(--color-fg);
    margin: 0;
    line-height: var(--line-height-tight);
  }

  .oq-abstract {
    font-size: var(--font-size-base);
    line-height: var(--line-height-loose);
    color: var(--color-fg-muted);
    margin: 0;
  }

  /* WP group */
  .wp-group {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .wp-group-header {
    display: flex;
    align-items: baseline;
    gap: var(--space-3);
    padding-bottom: var(--space-3);
    border-bottom: 2px solid var(--color-border);
  }

  .wp-group-id {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    font-weight: var(--font-weight-semibold);
    color: var(--color-accent);
    text-decoration: none;
    letter-spacing: 0.05em;
  }

  .wp-group-id:hover {
    text-decoration: underline;
  }

  .wp-group-title {
    font-size: var(--font-size-lg);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    margin: 0;
    flex: 1;
  }

  .wp-group-count {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    font-family: var(--font-mono);
  }

  /* Question list */
  .oq-list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
  }

  .oq-item {
    padding: var(--space-4);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .oq-item-head {
    display: flex;
    align-items: baseline;
    gap: var(--space-3);
    flex-wrap: wrap;
  }

  .oq-id {
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--color-fg-subtle);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }

  .oq-discipline {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    font-style: italic;
  }

  .oq-label {
    font-size: var(--font-size-base);
    font-weight: var(--font-weight-medium);
    color: var(--color-fg);
    margin: 0;
    line-height: var(--line-height-base);
  }

  .oq-question {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
    margin: 0;
  }

  .oq-deliverable,
  .oq-hook {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
    margin: 0;
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-md);
  }

  .oq-deliverable {
    background: rgba(82, 131, 184, 0.07);
    border-left: 2px solid var(--color-accent-muted);
  }

  .oq-hook {
    background: var(--color-bg-elevated);
    border-left: 2px solid var(--color-border);
  }

  .deliverable-label,
  .hook-label {
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    margin-right: var(--space-1);
  }

  .oq-footer {
    border-top: 1px solid var(--color-border);
    padding-top: var(--space-5);
  }

  .back-link {
    font-size: var(--font-size-sm);
    color: var(--color-accent);
    text-decoration: none;
  }

  .back-link:hover {
    text-decoration: underline;
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
