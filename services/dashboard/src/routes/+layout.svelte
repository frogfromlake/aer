<script lang="ts">
  import '$lib/design/global.css';
  import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
  import { setUnauthenticatedHandler } from '$lib/api/queries';
  import { handleUnauthenticated } from '$lib/state/auth.svelte';
  // Phase 144 — importing the locale rune applies `overwriteGetLocale` (its
  // module side-effect) before any `m.*()` message call renders on the client.
  import { locale } from '$lib/state/locale.svelte';
  import { m } from '$lib/paraglide/messages.js';
  let { children } = $props();

  // Keep <html lang> in sync with the active UI locale (a11y + correct
  // hyphenation/voicing). Prerendered HTML ships `lang="en"`; this corrects it
  // on the client when the resolved locale is German.
  $effect(() => {
    document.documentElement.lang = locale();
  });

  // Phase 134 / ADR-040: route any data-layer 401 to the auth redirect, without
  // the data layer importing the auth/navigation modules.
  setUnauthenticatedHandler(handleUnauthenticated);

  // One QueryClient for the lifetime of the shell. Defaults:
  //  - Refusals are returned as success data; only a thrown NetworkErrorOutcome
  //    retries. A 401 (known-dead session) already fired the bounce, so the
  //    retry predicate excludes it (SEC-087) — otherwise the default 3x backoff
  //    keeps hammering the now-401-ing BFF. 5xx/transport failures still retry 3x.
  //  - `refetchOnWindowFocus` is off: the Atmosphere holds long views;
  //    silent refetches would re-pulse the globe on every tab-switch.
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        refetchOnWindowFocus: false,
        retry: (failureCount, error) =>
          (error as { httpStatus?: number } | null)?.httpStatus !== 401 && failureCount < 3
      }
    }
  });
</script>

<QueryClientProvider {client}>
  <a href="#main" class="skip-link">{m.common_skip_to_main()}</a>
  <main id="main">
    {@render children()}
  </main>
</QueryClientProvider>

<style>
  main {
    min-height: 100vh;
    display: grid;
    place-items: center;
    padding: var(--space-5);
  }
</style>
