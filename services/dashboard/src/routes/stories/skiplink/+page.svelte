<script lang="ts">
  import SkipLink from '$lib/components/base/SkipLink.svelte';
</script>

<svelte:head>
  <title>SkipLink — AĒR Stories</title>
</svelte:head>

<h2>SkipLink</h2>
<p class="lede">
  Visually hidden until focused. Tab into the demo region below to reveal it. Default target is
  <code>#main</code>.
</p>

<section aria-labelledby="demo">
  <h3 id="demo">Demo</h3>
  <div class="demo">
    <SkipLink href="#skiplink-target" label="Skip to demo target" />
    <p>
      Press <kbd>Tab</kbd> with focus inside this region. The skip link will appear in the top-left of
      the viewport.
    </p>
    <div id="skiplink-target" tabindex="-1" class="target">Target region</div>
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
  .demo {
    padding: var(--space-5);
    border: 1px dashed var(--color-border);
    border-radius: var(--radius-md);
  }
  .target {
    margin-top: var(--space-4);
    padding: var(--space-4);
    background: var(--color-surface);
    border-radius: var(--radius-md);
  }
  kbd {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    padding: 2px 6px;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
  }
  code {
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
  }
</style>
