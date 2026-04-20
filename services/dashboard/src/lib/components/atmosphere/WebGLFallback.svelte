<script lang="ts">
  // Static fallback for browsers without WebGL2. Phase 99a's mandate is
  // narrow: avoid a broken screen, not deliver a feature. The proper
  // 2D Low-Fidelity atmosphere is its own future phase (see ROADMAP).
</script>

<section class="fallback" aria-labelledby="fallback-title">
  <h1 id="fallback-title">ἀήρ</h1>
  <p class="lede">Atmospheric sensor for human discourse.</p>
  <p class="notice">
    This version of AĒR requires WebGL2 for the full experience. A fully accessible alternative is
    planned and will appear here once it ships.
  </p>
</section>

<style>
  .fallback {
    text-align: center;
    max-width: 44rem;
    padding: var(--space-6);
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
  .notice {
    color: var(--color-fg-subtle);
    font-size: var(--font-size-sm);
    line-height: var(--line-height-loose);
    max-width: 36rem;
    margin: 0 auto;
  }
</style>
