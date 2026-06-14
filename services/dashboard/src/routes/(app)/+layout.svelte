<script lang="ts">
  /* eslint-disable svelte/no-navigation-without-resolve -- internal auth redirect (/login) */
  // Application chrome layout (authenticated surfaces).
  //
  // Renders the persistent left side rail (SideRail) + the top-right profile
  // menu. The per-surface chrome (PillarSwitch, WorkbenchScopeBar) is mounted
  // by the route page that owns them (e.g. /workbench/+page.svelte).
  //
  // Phase 134 / ADR-040: the whole app is gated. This layout checks the session
  // (GET /auth/me) before rendering any protected content; an unauthenticated
  // visitor is bounced to /login with a return path. Protected children are NOT
  // rendered until the session is confirmed (no flash of gated UI).
  //
  // This layout is NOT applied to /stories/* or the pre-auth pages (/login,
  // /accept-invite, /forgot-password, /reset-password): those live outside the
  // (app) group and inherit only the root layout (QueryClientProvider).
  //
  // afterNavigate re-hydrates the URL-backed rune store on every SPA
  // navigation. SvelteKit's <a> link interception uses pushState, which
  // does not fire `popstate`; without this hook, components reading
  // `urlState()` after a navigation see stale pre-navigation values.
  import type { Snippet } from 'svelte';
  import { onMount } from 'svelte';
  import { afterNavigate, goto } from '$app/navigation';
  import { SideRail } from '$lib/components/chrome';
  import AtmosphereSurface from '$lib/components/atmosphere/AtmosphereSurface.svelte';
  import DossierOverlay from '$lib/components/dossier/DossierOverlay.svelte';
  import AccountOverlay from '$lib/components/account/AccountOverlay.svelte';
  import AdminOverlay from '$lib/components/account/AdminOverlay.svelte';
  import AnalysesOverlay from '$lib/components/account/AnalysesOverlay.svelte';
  import { rehydrateUrlState } from '$lib/state/url.svelte';
  import { user, authChecked, refreshMe } from '$lib/state/auth.svelte';

  interface Props {
    children?: Snippet;
  }

  let { children }: Props = $props();

  let ready = $state(false);

  onMount(async () => {
    if (!authChecked()) await refreshMe();
    if (!user()) {
      const here = window.location.pathname + window.location.search;
      await goto(`/login?redirect=${encodeURIComponent(here)}`);
      return;
    }
    ready = true;
  });

  afterNavigate(() => {
    rehydrateUrlState();
  });
</script>

{#if ready && user()}
  <SideRail />
  <!-- Phase 135 — the Atmosphere globe is rendered persistently here so it
       survives navigation between surfaces and never remounts (it only reloads
       on a full page refresh). Its interactive chrome shows only on `/`; on the
       Workbench / Reflection it is a glassy backdrop behind the page content. -->
  <AtmosphereSurface />
  {#if children}{@render children()}{/if}
  <DossierOverlay />
  <AccountOverlay />
  <AdminOverlay />
  <AnalysesOverlay />
{/if}
