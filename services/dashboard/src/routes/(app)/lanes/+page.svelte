<script lang="ts">
  // Surface II — no probe selected.
  // Shown when the user navigates to /lanes without a probe ID.
  // Invites them to select a probe from the Atmosphere globe.
  import { ScopeBar } from '$lib/components/chrome';
  import { urlState } from '$lib/state/url.svelte';

  const url = $derived(urlState());
  let activeProbe = $derived(url.probe);
</script>

<svelte:head>
  <title>AĒR — Function Lanes</title>
</svelte:head>

<ScopeBar label="Surface II — Function Lanes navigation">
  <span class="surface-label">Function Lanes</span>
</ScopeBar>

<!-- eslint-disable svelte/no-navigation-without-resolve -- internal surface navigation links -->
<main class="no-probe" id="main-lanes">
  <div class="invite-card">
    <span class="invite-glyph" aria-hidden="true">◎</span>
    <h1 class="invite-title">Select a probe</h1>
    <p class="invite-body">
      Return to the Atmosphere surface and click a probe glyph to open its Dossier. Function Lanes,
      source cards, and article browsing open from there.
    </p>
    <a href="/" class="invite-link" data-sveltekit-preload-data="hover"> ← Back to Atmosphere </a>
    {#if activeProbe}
      <a
        href="/lanes/{activeProbe}/dossier"
        class="invite-link primary"
        data-sveltekit-preload-data="off"
      >
        Open Dossier for active probe
      </a>
    {/if}
  </div>
</main>

<style>
  .no-probe {
    position: fixed;
    inset: 0;
    left: var(--rail-width);
    top: var(--scope-bar-height);
    right: var(--tray-right-edge, var(--tray-closed-width));
    display: grid;
    place-items: center;
    background: var(--color-bg);
  }

  .invite-card {
    max-width: 26rem;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-4);
    padding: var(--space-7);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    text-align: center;
  }

  .invite-glyph {
    font-size: 2.5rem;
    color: var(--color-accent);
    line-height: 1;
  }

  .invite-title {
    font-size: var(--font-size-xl);
    font-weight: var(--font-weight-medium);
    color: var(--color-fg);
    margin: 0;
  }

  .invite-body {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
    margin: 0;
  }

  .invite-link {
    font-size: var(--font-size-sm);
    color: var(--color-accent-muted);
    text-decoration: none;
    padding: var(--space-2) var(--space-4);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    transition: all var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .invite-link:hover,
  .invite-link:focus-visible {
    color: var(--color-accent);
    border-color: var(--color-accent-muted);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .invite-link.primary {
    background: rgba(125, 220, 229, 0.08);
    border-color: var(--color-accent-muted);
    color: var(--color-accent);
  }

  .surface-label {
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-medium);
    color: var(--color-fg);
  }

  @media (prefers-reduced-motion: reduce) {
    .invite-link {
      transition: none;
    }
  }
</style>
