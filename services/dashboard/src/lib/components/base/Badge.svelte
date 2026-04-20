<script lang="ts">
  import type { Snippet } from 'svelte';

  type Tier =
    | 'tier1-unvalidated'
    | 'tier1-validated'
    | 'tier2-validated'
    | 'tier3'
    | 'expired'
    | 'refused';

  interface Props {
    tier?: Tier;
    label?: string;
    children?: Snippet;
  }

  let { tier = 'tier1-unvalidated', label, children }: Props = $props();

  const tierClass = $derived(`weight-${tier}`);
  const accessibleLabel = $derived(label ?? defaultLabel(tier));

  function defaultLabel(t: Tier): string {
    switch (t) {
      case 'tier1-unvalidated':
        return 'Tier 1, unvalidated';
      case 'tier1-validated':
        return 'Tier 1, validated';
      case 'tier2-validated':
        return 'Tier 2, validated';
      case 'tier3':
        return 'Tier 3, non-deterministic';
      case 'expired':
        return 'Validation expired';
      case 'refused':
        return 'Methodological refusal';
    }
  }
</script>

<span class="weight-badge {tierClass}" aria-label={accessibleLabel}>
  {#if children}
    {@render children()}
  {:else}
    <span aria-hidden="true">{accessibleLabel}</span>
  {/if}
</span>

<style>
  /* The visual treatment comes entirely from epistemic-weight.css. Only scope
     overrides that apply to .weight-badge inside this component live here — at
     the moment, none. The class is loaded globally via design/global.css so
     Badge does not need to re-import it. */

  .weight-badge.weight-refused {
    --weight-badge-color: var(--color-status-refused);
  }
</style>
