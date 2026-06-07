<script lang="ts">
  // NegativeSpaceBadge — the single visual primitive for Negative-Space class
  // display (Phase 122d.2 / ADR-039). One badge, used everywhere an NS-class
  // surfaces (ArticleRow, L5 reader, cell overlays). Mirrors FunctionBadge's
  // grammar so the dashboard's badge vocabulary is consistent.
  //
  // Register discipline (WP-006 §6.2): the styling is perceptually-neutral DIM,
  // never red/warning — absence invites a question, it does not assert a defect.
  // Colour + prose + WP anchor all come from NS_CLASS_DEFINITIONS (the SoT).
  import { getNSClassDef } from '$lib/negative-space';

  interface Props {
    /** NS-class key. Unknown keys render an inert grey badge (never crashes). */
    nsClass: string | null | undefined;
    size?: 'sm' | 'md' | 'lg';
    showLabel?: boolean;
    showInfo?: boolean;
  }

  let { nsClass, size = 'sm', showLabel = false, showInfo = false }: Props = $props();

  const def = $derived(getNSClassDef(nsClass));
</script>

<!-- eslint-disable svelte/no-navigation-without-resolve -- WP anchors are internal Reflection routes -->
{#if def}
  <span
    class="ns-badge size-{size}"
    style:--ns-color={def.color}
    role="img"
    aria-label="Negative space: {def.label}"
    title={def.description}
  >
    <span class="dot" aria-hidden="true">∅</span>
    <span class="abbr">{def.abbr}</span>
    {#if showLabel}
      <span class="label">{def.label}</span>
    {/if}
    {#if showInfo}
      <a
        class="info"
        href={def.wpAnchor}
        title="Methodology — {def.label}"
        aria-label="Methodology: {def.label}"
        onclick={(e) => e.stopPropagation()}
        data-sveltekit-preload-data="hover"
      >
        ⓘ
      </a>
    {/if}
  </span>
{:else}
  <span class="ns-badge size-{size} inert" role="img" aria-label="Unknown negative-space class">
    <span class="dot" aria-hidden="true">∅</span>
    <span class="abbr">—</span>
  </span>
{/if}

<style>
  .ns-badge {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    padding: 2px var(--space-2);
    /* Dashed border — the methodological-register signal for "absence", visually
       distinct from the solid FunctionBadge and never a warning colour. */
    border: 1px dashed color-mix(in srgb, var(--ns-color, var(--color-border)) 55%, transparent);
    border-radius: var(--radius-pill);
    background: color-mix(in srgb, var(--ns-color, var(--color-surface)) 8%, transparent);
    font-family: var(--font-mono);
    color: var(--color-fg-muted, var(--color-fg));
    line-height: 1.2;
    white-space: nowrap;
  }

  .ns-badge.inert {
    --ns-color: var(--color-fg-subtle);
    color: var(--color-fg-subtle);
  }

  .dot {
    color: var(--ns-color, var(--color-fg-subtle));
    font-size: 0.85em;
    flex-shrink: 0;
  }

  .abbr {
    font-weight: var(--font-weight-semibold);
    letter-spacing: 0.04em;
    color: var(--ns-color, inherit);
  }

  .label {
    color: var(--color-fg);
    font-family: var(--font-ui);
    font-weight: var(--font-weight-medium);
  }

  .info {
    color: var(--color-fg-subtle);
    text-decoration: none;
    border-bottom: 1px dotted transparent;
    margin-left: 2px;
  }
  .info:hover,
  .info:focus-visible {
    color: var(--color-accent);
    border-bottom-color: var(--color-accent);
    outline: none;
  }

  .size-sm {
    font-size: 10px;
    padding: 1px var(--space-1);
  }
  .size-md {
    font-size: var(--font-size-xs);
  }
  .size-lg {
    font-size: var(--font-size-sm);
    padding: var(--space-1) var(--space-3);
  }
</style>
