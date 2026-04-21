<script lang="ts">
  import '$lib/design/global.css';
  import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
  let { children } = $props();

  // One QueryClient for the lifetime of the shell. Defaults:
  //  - Refusals are returned as success data, so `retry` on errors stays
  //    at TanStack's default (3x) — only 5xx/transport failures retry.
  //  - `refetchOnWindowFocus` is off: the Atmosphere holds long views;
  //    silent refetches would re-pulse the globe on every tab-switch.
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        refetchOnWindowFocus: false
      }
    }
  });
</script>

<QueryClientProvider {client}>
  <a href="#main" class="skip-link">Skip to main content</a>
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
