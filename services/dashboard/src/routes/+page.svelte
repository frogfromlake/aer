<svelte:head>
  <title>AĒR — Dashboard</title>
</svelte:head>

<section>
  <h1>ἀήρ</h1>
  <p class="lede">Atmospheric sensor for human discourse.</p>
  <p class="phase">Phase 98 design-system foundation. No user-facing features yet.</p>
</section>

<style>
  section {
    text-align: center;
    max-width: 40rem;
  }

  h1 {
    font-size: var(--font-size-4xl);
    margin: 0 0 var(--space-4);
    color: var(--color-fg);
  }

  .lede {
    color: var(--color-fg-muted);
    font-size: var(--font-size-md);
    margin: 0 0 var(--space-6);
  }

  .phase {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
  }
</style>
