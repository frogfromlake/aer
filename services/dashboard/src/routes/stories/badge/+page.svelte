<script lang="ts">
  import Badge from '$lib/components/base/Badge.svelte';
</script>

<svelte:head>
  <title>Badge — AĒR Stories</title>
</svelte:head>

<h2>Badge</h2>
<p class="lede">
  Epistemic Weight tier indicators per Design Brief §5.8. Each badge carries an accessible label so
  tier is never conveyed by color alone.
</p>

<section aria-labelledby="tiers">
  <h3 id="tiers">Tiers</h3>
  <div class="row">
    <Badge tier="tier1-unvalidated" />
    <Badge tier="tier1-validated" />
    <Badge tier="tier2-validated" />
    <Badge tier="tier3" />
    <Badge tier="expired" />
    <Badge tier="refused" />
  </div>
</section>

<section aria-labelledby="custom">
  <h3 id="custom">Custom label</h3>
  <div class="row">
    <Badge tier="tier2-validated" label="Sentiment (SentiWS v2.0)">Sentiment · v2.0</Badge>
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
