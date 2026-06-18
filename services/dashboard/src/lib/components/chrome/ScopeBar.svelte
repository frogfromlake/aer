<script lang="ts">
  // Top scope bar — Design Brief §3.2.
  // Slot-based horizontal strip accepting per-surface navigation content:
  //   Surface I  — time window label + resolution selector + Neg-Space toggle
  //   Surface II — lane switcher (Phase 106)
  //   Surface III — section anchor / reading progress (Phase 109)
  //
  // Positioned left of the side rail and right of the tray's right edge.
  // The `--tray-right-edge` custom property (updated by MethodologyTray when
  // the tray opens in push-mode) governs the right inset so scope bar and
  // tray never overlap.
  //
  // Phase 113: a leading "Surface · Layer" chip exposes the descent
  // vocabulary (Brief §4.1) so users can locate themselves without
  // hovering rail tooltips.
  import type { Snippet } from 'svelte';
  import { page } from '$app/state';
  import { m } from '$lib/paraglide/messages.js';

  interface Props {
    children?: Snippet;
    /** Accessible label for the navigation region. */
    label?: string;
  }

  let { children, label }: Props = $props();

  // Derive surface and layer from the route so the chip stays accurate as the
  // user descends. ScopeBar renders only on Reflection routes today, so the
  // Reflection branch is the live one; the Atmosphere fallback is the default.
  const breadcrumb = $derived.by(() => {
    const p = page.url.pathname;
    if (p.startsWith('/reflection')) {
      return {
        surface: 'Surface III',
        surfaceName: m.chrome_surface_reflection(),
        layer: 'L1',
        layerName: m.chrome_layer_working_papers()
      };
    }
    return {
      surface: 'Surface I',
      surfaceName: m.chrome_surface_atmosphere(),
      layer: 'L0',
      layerName: m.chrome_layer_globe()
    };
  });
</script>

<div class="scope-bar" role="navigation" aria-label={label ?? m.chrome_scopebar_nav_label()}>
  <div class="inner">
    <span
      class="surface-chip"
      aria-label={m.chrome_scopebar_chip_aria({
        surface: breadcrumb.surface,
        surfaceName: breadcrumb.surfaceName,
        layer: breadcrumb.layer,
        layerName: breadcrumb.layerName
      })}
      title={m.chrome_scopebar_chip_title()}
    >
      <span class="chip-prefix">{breadcrumb.surface}</span>
      <span class="chip-sep" aria-hidden="true">·</span>
      <span class="chip-name">{breadcrumb.surfaceName}</span>
      <span class="chip-sep" aria-hidden="true">·</span>
      <span class="chip-layer">{breadcrumb.layer}</span>
      <span class="chip-layer-name">{breadcrumb.layerName}</span>
    </span>
    {#if children}{@render children()}{/if}
  </div>
</div>

<style>
  .scope-bar {
    position: fixed;
    top: 0;
    left: var(--rail-width);
    right: var(--tray-right-edge, var(--tray-closed-width));
    z-index: 440;
    background: var(--color-bg-overlay);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    border-bottom: 1px solid var(--color-border);
    min-height: var(--scope-bar-height);
  }

  .inner {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: 0 var(--space-4);
    min-height: var(--scope-bar-height);
    flex-wrap: wrap;
  }

  .surface-chip {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    padding: 2px var(--space-2);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-pill);
    background: var(--color-bg-elevated);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--color-fg-muted);
    font-family: var(--font-ui);
    white-space: nowrap;
    cursor: default;
    flex-shrink: 0;
  }

  .chip-prefix {
    color: var(--color-accent);
    font-weight: var(--font-weight-semibold);
  }

  .chip-name {
    color: var(--color-fg);
  }

  .chip-sep {
    opacity: 0.5;
  }

  .chip-layer {
    color: var(--color-accent);
    font-family: var(--font-mono);
    font-weight: var(--font-weight-semibold);
  }

  .chip-layer-name {
    color: var(--color-fg-muted);
  }
</style>
