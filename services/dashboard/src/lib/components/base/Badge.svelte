<script lang="ts">
  // Badge — generic provenance/status pill (validation tier, expired, refused).
  // The tonal primitive the metric-provenance badges build on.
  import type { Snippet } from 'svelte';
  import { m } from '$lib/paraglide/messages.js';

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
        return m.base_badge_tier1_unvalidated();
      case 'tier1-validated':
        return m.base_badge_tier1_validated();
      case 'tier2-validated':
        return m.base_badge_tier2_validated();
      case 'tier3':
        return m.base_badge_tier3();
      case 'expired':
        return m.base_badge_expired();
      case 'refused':
        return m.base_badge_refused();
    }
  }
</script>

<span class="weight-badge {tierClass}" role="img" aria-label={accessibleLabel}>
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
