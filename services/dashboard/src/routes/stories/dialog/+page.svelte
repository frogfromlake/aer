<script lang="ts">
  import Dialog from '$lib/components/base/Dialog.svelte';
  import Button from '$lib/components/base/Button.svelte';

  let open = $state(false);
  let openWithFooter = $state(false);
</script>

<svelte:head>
  <title>Dialog — AĒR Stories</title>
</svelte:head>

<h2>Dialog</h2>
<p class="lede">
  Modal dialog with focus trap, Escape to close, backdrop click to close, and focus restoration on
  close.
</p>

<section aria-labelledby="basic">
  <h3 id="basic">Basic</h3>
  <Button onclick={() => (open = true)}>Open dialog</Button>
  <Dialog bind:open title="Provisional metric">
    <p>
      Sentiment is computed with SentiWS v2.0. Results are Tier 2, validated — not scientifically
      validated measurements.
    </p>
  </Dialog>
</section>

<section aria-labelledby="with-footer">
  <h3 id="with-footer">With footer actions</h3>
  <Button onclick={() => (openWithFooter = true)}>Open with footer</Button>
  <Dialog bind:open={openWithFooter} title="Confirm refusal">
    <p>Mark this metric as methodologically refused?</p>
    {#snippet footer()}
      <Button variant="ghost" onclick={() => (openWithFooter = false)}>Cancel</Button>
      <Button onclick={() => (openWithFooter = false)}>Confirm</Button>
    {/snippet}
  </Dialog>
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
</style>
