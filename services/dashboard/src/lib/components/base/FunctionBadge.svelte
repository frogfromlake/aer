<script lang="ts">
  // FunctionBadge — single visual primitive for discourse-function display
  // (Phase 122h / ADR-033 §4). One Badge, two Pillar-anchored locations:
  // ProbeDossier Source-Cards, Cell-Header
  // filter strips. Replaces four inline-defined badge variants in the lanes/
  // tree.
  //
  // Variants:
  //   `size`         sm | md | lg — controls dot diameter, padding, font
  //   `showLabel`    when false renders only the dot + abbreviation
  //   `showInfo`     when true appends an ⓘ link to /reflection/wp/wp-001 §3
  //   `selected`     when true renders a filled background (chip-style for
  //                  selected scope filters); default false renders outline.
  //
  // Color is read from FUNCTION_DEFINITIONS so all four functions share the
  // same accent contract across the dashboard.
  import {
    getFunctionDef,
    FUNCTION_INFO_HREF,
    type DiscourseFunction
  } from '$lib/discourse-function';
  import { m } from '$lib/paraglide/messages.js';

  interface Props {
    /** Discourse-function key. Unknown keys render an inert grey badge so the
     *  UI never crashes on partial data. */
    function: DiscourseFunction | string | null | undefined;
    size?: 'sm' | 'md' | 'lg';
    showLabel?: boolean;
    showInfo?: boolean;
    selected?: boolean;
  }

  let {
    function: fnKey,
    size = 'md',
    showLabel = true,
    showInfo = false,
    selected = false
  }: Props = $props();

  const def = $derived(getFunctionDef(fnKey));
</script>

<!-- eslint-disable svelte/no-navigation-without-resolve -- WP-001 §3 anchor is an internal Reflection route -->
{#if def}
  <span
    class="function-badge size-{size}"
    class:selected
    style:--fn-color={def.color}
    role="img"
    aria-label={def.label}
    title={def.description}
  >
    <span class="dot" aria-hidden="true"></span>
    <span class="abbr">{def.abbr}</span>
    {#if showLabel}
      <span class="label">{def.label}</span>
    {/if}
    {#if showInfo}
      <a
        class="info"
        href={FUNCTION_INFO_HREF}
        title={m.base_function_badge_title({ label: def.label })}
        aria-label={m.base_function_badge_aria({ label: def.label })}
        onclick={(e) => e.stopPropagation()}
        data-sveltekit-preload-data="hover"
      >
        ⓘ
      </a>
    {/if}
  </span>
{:else}
  <span
    class="function-badge size-{size} inert"
    role="img"
    aria-label={m.base_function_badge_unknown()}
  >
    <span class="dot" aria-hidden="true"></span>
    <span class="abbr">—</span>
  </span>
{/if}

<style>
  .function-badge {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    padding: 2px var(--space-2);
    border: 1px solid color-mix(in srgb, var(--fn-color, var(--color-border)) 60%, transparent);
    border-radius: var(--radius-pill);
    background: color-mix(in srgb, var(--fn-color, var(--color-surface)) 10%, transparent);
    font-family: var(--font-mono);
    color: var(--color-fg);
    line-height: 1.2;
    white-space: nowrap;
  }

  .function-badge.selected {
    background: color-mix(in srgb, var(--fn-color) 22%, transparent);
    border-color: var(--fn-color);
  }

  .function-badge.inert {
    --fn-color: var(--color-fg-subtle);
    color: var(--color-fg-subtle);
  }

  .dot {
    width: 0.5rem;
    height: 0.5rem;
    border-radius: 50%;
    background: var(--fn-color, var(--color-fg-subtle));
    flex-shrink: 0;
  }

  .abbr {
    font-weight: var(--font-weight-semibold);
    letter-spacing: 0.04em;
    color: var(--fn-color, inherit);
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

  /* Sizes */
  .size-sm {
    font-size: 10px;
    padding: 1px var(--space-1);
  }
  .size-sm .dot {
    width: 0.375rem;
    height: 0.375rem;
  }

  .size-md {
    font-size: var(--font-size-xs);
  }

  .size-lg {
    font-size: var(--font-size-sm);
    padding: var(--space-1) var(--space-3);
  }
  .size-lg .dot {
    width: 0.625rem;
    height: 0.625rem;
  }
</style>
