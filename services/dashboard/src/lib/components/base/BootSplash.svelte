<script lang="ts">
  // App-bootstrap splash — the ONE full-screen loading state shown while AĒR
  // boots (session probe + lazy 3D engine), before the globe is interactive.
  //
  // It reuses the atmospheric "breathing" motif (CellLoadingState) so the boot
  // wait reads in the same visual language as every in-app loading cell, and
  // adds the AĒR wordmark + a localized label over the globe-black backdrop.
  //
  // Anti-flicker is the whole job (see ./boot-timing): the splash is WITHHELD
  // for SHOW_DELAY_MS, so a fast load never flashes it; once shown it holds for
  // MIN_VISIBLE_MS, so it never blinks out; then it cross-fades onto the globe.
  // A requestAnimationFrame loop advances the clock only until the phase reaches
  // `done`, then stops — no idle ticking once boot settles. `prefers-reduced-
  // motion` is inherited from CellLoadingState (still set, no movement).
  import { onMount } from 'svelte';
  import CellLoadingState from './CellLoadingState.svelte';
  import { bootReady, splashPhase, splashVisible } from '$lib/state/boot.svelte';
  import { m } from '$lib/paraglide/messages.js';

  // One clock for every timing input. `performance.now()` is monotonic and
  // available in the browser; the splash logic only subtracts timestamps.
  const now = () => (typeof performance !== 'undefined' ? performance.now() : 0);
  const mountMs = now();

  let nowMs = $state(mountMs);
  let readyMs = $state<number | null>(null);

  // Latch the instant boot completed (once; monotonic).
  $effect(() => {
    if (bootReady() && readyMs === null) readyMs = now();
  });

  const phase = $derived(splashPhase(mountMs, readyMs, nowMs));

  onMount(() => {
    let raf = 0;
    const tick = () => {
      nowMs = now();
      if (splashPhase(mountMs, readyMs, nowMs) === 'done') return; // settle → stop
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  });
</script>

{#if splashVisible(phase)}
  <div class="boot-splash" class:fading={phase === 'fading'}>
    <div class="boot-inner">
      <span class="boot-mark" aria-hidden="true">AĒR</span>
      <CellLoadingState label={m.boot_loading_label()} />
    </div>
  </div>
{/if}

<style>
  .boot-splash {
    position: fixed;
    inset: 0;
    /* Globe-black so the fade lands on the same colour the engine paints over. */
    background: #000;
    display: grid;
    place-items: center;
    /* Above every surface (SideRail 450, overlays 40, tooltips 600, Zen 2000). */
    z-index: 5000;
    opacity: 1;
    transition: opacity 320ms ease-in-out;
  }
  .boot-splash.fading {
    opacity: 0;
  }

  .boot-inner {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-4);
  }

  .boot-mark {
    font-family: var(--font-mono);
    font-size: var(--font-size-2xl, 1.5rem);
    letter-spacing: 0.18em;
    color: var(--color-fg);
    opacity: 0.92;
  }

  @media (prefers-reduced-motion: reduce) {
    .boot-splash {
      /* Still cross-fade, but instantly — no lingering animated opacity. */
      transition: opacity 1ms linear;
    }
  }
</style>
