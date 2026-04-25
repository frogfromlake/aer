<script lang="ts">
  // Application chrome layout (Phase 105 — Navigation Chrome).
  // Renders the three-part persistent frame from Design Brief §3.2:
  //   Left side rail  — SideRail (surface anchors, scope indicator, pillar toggle)
  //   Right method. tray — MethodologyTray (L4 Provenance, always reachable)
  //
  // The top scope bar (ScopeBar) is surface-specific and is rendered by
  // each surface page, not here — its content changes per surface:
  //   Surface I   — time window + resolution + Negative Space toggle
  //   Surface II  — lane switcher (Phase 106)
  //   Surface III — section anchor (Phase 109)
  //
  // This layout is NOT applied to /stories/* routes: those live outside the
  // (app) group and inherit only the root layout (QueryClientProvider).
  import type { Snippet } from 'svelte';
  import { SideRail, MethodologyTray } from '$lib/components/chrome';

  interface Props {
    children?: Snippet;
  }

  let { children }: Props = $props();
</script>

<SideRail />
{#if children}{@render children()}{/if}
<MethodologyTray />
