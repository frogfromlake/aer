<script lang="ts">
  import Button from '$lib/components/base/Button.svelte';
</script>

<svelte:head>
  <title>Button — AĒR Stories</title>
</svelte:head>

<h2>Button</h2>
<p class="lede">
  Primary, secondary, and ghost variants. Two sizes. Loading and disabled states per variant.
</p>

<section aria-labelledby="variants">
  <h3 id="variants">Variants</h3>
  <div class="row">
    <Button variant="primary">Primary</Button>
    <Button variant="secondary">Secondary</Button>
    <Button variant="ghost">Ghost</Button>
  </div>
</section>

<section aria-labelledby="sizes">
  <h3 id="sizes">Sizes</h3>
  <div class="row">
    <Button size="sm">Small</Button>
    <Button size="md">Medium</Button>
  </div>
</section>

<section aria-labelledby="states">
  <h3 id="states">States</h3>
  <div class="row">
    <Button loading>Loading</Button>
    <Button disabled>Disabled</Button>
    <Button variant="secondary" loading>Loading</Button>
    <Button variant="secondary" disabled>Disabled</Button>
    <Button variant="ghost" loading>Loading</Button>
    <Button variant="ghost" disabled>Disabled</Button>
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
  .row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-3);
    align-items: center;
  }
</style>
