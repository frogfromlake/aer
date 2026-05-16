<script lang="ts">
  // Application chrome layout.
  //
  // Renders the persistent left side rail (SideRail). The per-surface
  // chrome (PillarSwitch, WorkbenchScopeBar) is mounted by the route
  // page that owns them (e.g. /workbench/+page.svelte).
  //
  // This layout is NOT applied to /stories/* routes: those live outside the
  // (app) group and inherit only the root layout (QueryClientProvider).
  //
  // afterNavigate re-hydrates the URL-backed rune store on every SPA
  // navigation. SvelteKit's <a> link interception uses pushState, which
  // does not fire `popstate`; without this hook, components reading
  // `urlState()` after a navigation see stale pre-navigation values.
  import type { Snippet } from 'svelte';
  import { afterNavigate } from '$app/navigation';
  import { SideRail } from '$lib/components/chrome';
  import { rehydrateUrlState } from '$lib/state/url.svelte';

  interface Props {
    children?: Snippet;
  }

  let { children }: Props = $props();

  afterNavigate(() => {
    rehydrateUrlState();
  });
</script>

<SideRail />
{#if children}{@render children()}{/if}
