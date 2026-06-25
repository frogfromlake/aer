<script lang="ts">
  /* eslint-disable svelte/no-navigation-without-resolve -- internal auth redirect (/login) */
  // Application chrome layout (authenticated surfaces).
  //
  // Renders the persistent left side rail (SideRail) + the top-right profile
  // menu. The per-surface chrome (PillarSwitch) is mounted
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
  import AnalysesOverlay from '$lib/components/account/AnalysesOverlay.svelte';
  import AboutOverlay from '$lib/components/about/AboutOverlay.svelte';
  import BootSplash from '$lib/components/base/BootSplash.svelte';
  import { rehydrateUrlState } from '$lib/state/url.svelte';
  import { markSessionReady } from '$lib/state/boot.svelte';
  import { user, authChecked, refreshMe } from '$lib/state/auth.svelte';
  import type { MeResult } from '$lib/api/auth';
  import Button from '$lib/components/base/Button.svelte';
  import { m } from '$lib/paraglide/messages.js';

  interface Props {
    children?: Snippet;
  }

  let { children }: Props = $props();

  let ready = $state(false);
  let unreachable = $state(false);

  // SEC-081: a transient BFF failure (offline / 5xx) must NOT be read as
  // logged-out. resolveSession bounces to /login ONLY on a definitive 401; an
  // inconclusive ('unknown') probe is retried in place with backoff, and if the
  // BFF stays unreachable the user is held on a manual-retry affordance rather
  // than evicted from a valid __Host- session.
  const MAX_RETRIES = 4;
  const RETRY_BASE_MS = 800;
  const delay = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

  async function resolveSession(): Promise<void> {
    unreachable = false;
    let state: MeResult['state'] = authChecked()
      ? user()
        ? 'authenticated'
        : 'unauthenticated'
      : (await refreshMe()).state;

    for (let attempt = 0; state === 'unknown' && attempt < MAX_RETRIES; attempt++) {
      await delay(RETRY_BASE_MS * 2 ** attempt);
      state = (await refreshMe()).state;
    }

    // Every terminal branch releases the boot splash's session gate — the page
    // stops being blank here (renders the app, the /login bounce, or the retry
    // surface), so the splash must not keep covering any of them.
    if (state === 'authenticated') {
      ready = true;
      markSessionReady();
      return;
    }
    if (state === 'unauthenticated') {
      const here = window.location.pathname + window.location.search;
      // `replaceState` so the unauthenticated bounce to /login does not pile an
      // auth entry onto the history stack; after sign-in the login page replaces
      // itself with the redirect target, keeping auth pages out of back/forward.
      markSessionReady();
      await goto(`/login?redirect=${encodeURIComponent(here)}`, { replaceState: true });
      return;
    }
    // Still inconclusive after the retries — keep the user in place (do not
    // bounce a possibly-valid session) and offer a manual retry.
    unreachable = true;
    markSessionReady();
  }

  onMount(resolveSession);

  afterNavigate(() => {
    rehydrateUrlState();
  });
</script>

<!-- Boot splash — self-gating (anti-flicker timing inside); always mounted so it
     covers the blank session-probe + engine-load window and clears once the
     globe is interactive. -->
<BootSplash />

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
  <AnalysesOverlay />
  <AboutOverlay />
{:else if unreachable}
  <div class="session-retry" role="alert">
    <p>{m.auth_session_unreachable()}</p>
    <Button onclick={resolveSession}>{m.common_retry()}</Button>
  </div>
{/if}

<style>
  .session-retry {
    min-height: 100vh;
    display: grid;
    place-items: center;
    align-content: center;
    gap: var(--space-4);
    padding: var(--space-5);
    text-align: center;
    color: var(--color-fg-muted);
  }
</style>
