<script lang="ts">
  // L1 Orientation overlay (Design Brief §4.2 — "a thin register of
  // where-you-are"). Soft top-bar that surfaces the current orientation
  // without pulling focus: time window, active probe count with
  // languages, normalization mode, and the emic designations of active
  // probes — sourced from the Content Catalog (/content/probe/{id}),
  // never hardcoded in frontend copy (WP-001 §4.2 + Arc42 §8.18).
  //
  // Visibility model: hidden on mount; fades in on first pointer
  // movement or keyboard input; fades out after 10s of inactivity.
  // Users can also focus the L1 bar via `Shift+Tab` from the globe to
  // force it visible (the overlay traps its own focus while active).
  import { onDestroy, onMount } from 'svelte';
  import type { ProbeDto } from '$lib/api/queries';
  import type { FetchContext } from '$lib/api/queries';

  interface Props {
    probes: readonly ProbeDto[];
    windowLabel: string;
    /** Currently applied normalization on /metrics ("raw" by default). */
    normalization: string;
    /** Shared BFF fetch context — used to pull emic content. */
    ctx: FetchContext;
  }

  let { probes, windowLabel, normalization, ctx }: Props = $props();

  let visible = $state(false);
  let forced = $state(false);
  let fadeTimer: ReturnType<typeof setTimeout> | null = null;

  // Emic short descriptions by probeId — loaded lazily from the Content
  // Catalog. Missing entries render as language-only so the overlay
  // never lies about what it knows.
  let emicShort: Record<string, string> = $state({});

  const FADE_OUT_MS = 10_000;

  function reveal() {
    visible = true;
    if (fadeTimer) clearTimeout(fadeTimer);
    if (!forced) {
      fadeTimer = setTimeout(() => {
        if (!forced) visible = false;
      }, FADE_OUT_MS);
    }
  }

  function onFirstInteraction() {
    reveal();
  }

  onMount(() => {
    window.addEventListener('pointermove', onFirstInteraction);
    window.addEventListener('keydown', onFirstInteraction);
    return () => {
      window.removeEventListener('pointermove', onFirstInteraction);
      window.removeEventListener('keydown', onFirstInteraction);
    };
  });

  onDestroy(() => {
    if (fadeTimer) clearTimeout(fadeTimer);
  });

  // Fetch emic content per probe. Failures are silently skipped — L1
  // orientation must never block L0 on a Content Catalog miss.
  $effect(() => {
    const pending = probes.filter((p) => !(p.probeId in emicShort));
    if (pending.length === 0) return;
    const doFetch = ctx.fetch ?? fetch;
    const base = ctx.baseUrl.replace(/\/$/, '');
    for (const probe of pending) {
      // UI-default locale: L1 text is English regardless of the probe's
      // source language — the source language is already surfaced via
      // the language chip next to each designation.
      doFetch(`${base}/content/probe/${encodeURIComponent(probe.probeId)}?locale=en`)
        .then((r) => (r.ok ? r.json() : null))
        .then((body: unknown) => {
          if (!body || typeof body !== 'object') return;
          const registers = (body as { registers?: { semantic?: { short?: string } } }).registers;
          const short = registers?.semantic?.short;
          if (typeof short === 'string') emicShort = { ...emicShort, [probe.probeId]: short };
        })
        .catch(() => void 0);
    }
  });

  let languages = $derived(Array.from(new Set(probes.map((p) => p.language.toUpperCase()))).sort());
</script>

<!--
  role=region / aria-label anchors the overlay for screen readers so
  Shift+Tab from the globe lands here as an orientation landmark.
-->
<div
  class="l1"
  class:visible
  role="region"
  aria-label="Orientation"
  tabindex="-1"
  onfocusin={() => {
    forced = true;
    reveal();
  }}
  onfocusout={() => {
    forced = false;
    reveal();
  }}
  onpointerenter={() => {
    forced = true;
    reveal();
  }}
  onpointerleave={() => {
    forced = false;
    reveal();
  }}
>
  <dl>
    <div class="cell">
      <dt>Window</dt>
      <dd>{windowLabel}</dd>
    </div>
    <div class="cell">
      <dt>Probes active</dt>
      <dd>{probes.length} ({languages.join(', ') || '—'})</dd>
    </div>
    <div class="cell">
      <dt>Normalization</dt>
      <dd>{normalization}</dd>
    </div>
    <div class="cell wide">
      <dt>Contexts</dt>
      <dd class="emic">
        {#if probes.length === 0}
          <span class="muted">no active probes</span>
        {:else}
          <ul>
            {#each probes as p (p.probeId)}
              <li>
                <span class="lang">{p.language.toUpperCase()}</span>
                <span class="designation"
                  >{emicShort[p.probeId] ?? `emic designation loading…`}</span
                >
              </li>
            {/each}
          </ul>
        {/if}
      </dd>
    </div>
  </dl>
</div>

<style>
  .l1 {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    z-index: 450;
    padding: var(--space-3) var(--space-5);
    background: linear-gradient(to bottom, rgba(0, 0, 0, 0.55), rgba(0, 0, 0, 0));
    color: var(--color-fg);
    font-size: var(--font-size-xs);
    opacity: 0;
    pointer-events: none;
    transition: opacity 420ms ease-in-out;
  }
  .l1.visible {
    opacity: 1;
    pointer-events: auto;
  }
  .l1:focus-within {
    outline: 2px solid var(--color-accent);
    outline-offset: -2px;
  }
  dl {
    display: grid;
    grid-template-columns: repeat(3, auto) 1fr;
    gap: var(--space-1) var(--space-5);
    margin: 0;
    align-items: start;
  }
  .cell {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .cell.wide {
    min-width: 0;
  }
  dt {
    color: var(--color-fg-subtle);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    font-size: 10px;
  }
  dd {
    margin: 0;
    font-family: var(--font-family-mono);
  }
  .emic ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
    font-size: 11px;
  }
  .emic .lang {
    display: inline-block;
    margin-right: var(--space-2);
    padding: 0 4px;
    border: 1px solid var(--color-border);
    border-radius: 3px;
    color: var(--color-fg-muted);
    text-transform: uppercase;
  }
  .emic .designation {
    color: var(--color-fg);
    font-family: var(--font-family-sans, inherit);
  }
  .muted {
    color: var(--color-fg-subtle);
  }
</style>
