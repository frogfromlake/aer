<script lang="ts">
  // Dual-Register primitive (Design Brief §5.7).
  //
  // Every entity the dashboard shows — a metric, a probe, a discourse
  // function, a refusal — has two authored registers: a plain-language
  // *semantic* register aimed at a first-time reader, and a precise
  // *methodological* register that names the gate, the algorithm, or
  // the WP anchor.
  //
  // At most surfaces the semantic register is primary; the
  // methodological register is accessible via a compact toggle. Both
  // registers are always present in the DOM so a screen-reader user can
  // feel the methodological register with Tab just as easily as a
  // sighted user can click the toggle; CSS controls visual prominence,
  // not presence.
  //
  // This primitive takes a `ContentRegisters` payload (already fetched
  // from `/content/...`) and renders it. It does not fetch anything
  // itself — that is the caller's responsibility. A separate variant
  // (`emphasis='methodological'`) flips the visual weighting at L4.
  import type { components } from '$lib/api/types';

  type Registers = components['schemas']['ContentRegisters'];

  interface Props {
    registers: Registers;
    /** Which register is the default primary surface. Flip at L4. */
    emphasis?: 'semantic' | 'methodological';
    /** Pass through a stable id so labels elsewhere can point at this block. */
    id?: string;
    /** Render the secondary register as "short" (hover-tooltip scale) or "long" (full paragraph). */
    detail?: 'short' | 'long';
  }

  let { registers, emphasis = 'semantic', id, detail = 'long' }: Props = $props();

  let expanded = $state(false);
  const toggleId = `ps-toggle-${Math.random().toString(36).slice(2, 10)}`;
  const fallbackRegionId = `ps-region-${Math.random().toString(36).slice(2, 10)}`;
  let regionId = $derived(id ?? fallbackRegionId);

  let primary = $derived(emphasis === 'semantic' ? registers.semantic : registers.methodological);
  let secondary = $derived(emphasis === 'semantic' ? registers.methodological : registers.semantic);
  let secondaryLabel = $derived(emphasis === 'semantic' ? 'Methodological' : 'Plain language');
</script>

<div class="progressive-semantics" {id}>
  <p class="primary">{primary.long}</p>
  <button
    type="button"
    class="toggle"
    id={toggleId}
    aria-expanded={expanded}
    aria-controls={regionId}
    onclick={() => (expanded = !expanded)}
  >
    {secondaryLabel} register
  </button>
  <div
    id={regionId}
    class="secondary"
    class:visible={expanded}
    role="region"
    aria-labelledby={toggleId}
    hidden={!expanded}
  >
    <p>{detail === 'short' ? secondary.short : secondary.long}</p>
  </div>
</div>

<style>
  .progressive-semantics {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .primary {
    margin: 0;
    color: var(--color-fg);
    font-size: var(--font-size-base);
    line-height: 1.55;
  }
  .toggle {
    align-self: flex-start;
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg-muted);
    font-size: var(--font-size-xs);
    padding: var(--space-1) var(--space-3);
    cursor: pointer;
  }
  .toggle:hover,
  .toggle:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-accent);
  }
  .toggle[aria-expanded='true'] {
    color: var(--color-fg);
    border-color: var(--color-accent);
  }
  .secondary {
    border-left: 2px solid var(--color-border);
    padding-left: var(--space-3);
    color: var(--color-fg-muted);
    font-size: var(--font-size-sm);
    line-height: 1.55;
  }
  .secondary p {
    margin: 0;
  }
</style>
