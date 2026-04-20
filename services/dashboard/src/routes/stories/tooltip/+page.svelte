<script lang="ts">
  import Tooltip from '$lib/components/base/Tooltip.svelte';
  import Button from '$lib/components/base/Button.svelte';
</script>

<svelte:head>
  <title>Tooltip — AĒR Stories</title>
</svelte:head>

<h2>Tooltip</h2>
<p class="lede">
  Supplementary text on hover or focus. Four placements. Never the sole carrier of information.
</p>

<section aria-labelledby="placements">
  <h3 id="placements">Placements</h3>
  <div class="grid">
    <Tooltip text="Tooltip on top" placement="top">
      <Button variant="secondary">Top</Button>
    </Tooltip>
    <Tooltip text="Tooltip on bottom" placement="bottom">
      <Button variant="secondary">Bottom</Button>
    </Tooltip>
    <Tooltip text="Tooltip on left" placement="left">
      <Button variant="secondary">Left</Button>
    </Tooltip>
    <Tooltip text="Tooltip on right" placement="right">
      <Button variant="secondary">Right</Button>
    </Tooltip>
  </div>
</section>

<style>
  h2 {
    font-size: var(--font-size-2xl);
    margin: 0 0 var(--space-2);
  }
  .lede {
    color: var(--color-fg-muted);
    margin: 0 0 var(--space-6);
    max-width: 60ch;
  }
  section {
    margin-bottom: var(--space-6);
  }
  h3 {
    font-size: var(--font-size-sm);
    text-transform: uppercase;
    letter-spacing: var(--letter-spacing-wide);
    color: var(--color-fg-subtle);
    margin: 0 0 var(--space-3);
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(2, max-content);
    gap: var(--space-8) var(--space-6);
    padding: var(--space-6);
  }
</style>
