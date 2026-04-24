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
  import { SegmentedControl } from '$lib/components/base';

  type Registers = components['schemas']['ContentRegisters'];
  type RegisterId = 'semantic' | 'methodological';

  interface Props {
    registers: Registers;
    /** Which register is the default primary surface. Flip at L4. */
    emphasis?: RegisterId;
    /** Pass through a stable id so labels elsewhere can point at this block. */
    id?: string;
    /** Render the secondary register as "short" (hover-tooltip scale) or "long" (full paragraph). */
    detail?: 'short' | 'long';
  }

  let { registers, emphasis = 'semantic', id, detail = 'long' }: Props = $props();

  // Active register is user-controllable via the segmented toggle. We
  // initialise to `emphasis` so L3 lands on plain language and L4 used
  // to land on methodological — but the reader can always flip at any
  // surface without losing either register (both stay in the DOM for a
  // screen-reader pass).
  let active: RegisterId = $state(emphasis);
  const fallbackRegionId = `ps-region-${Math.random().toString(36).slice(2, 10)}`;
  let regionId = $derived(id ?? fallbackRegionId);

  const OPTIONS: readonly { id: RegisterId; label: string; hint: string }[] = [
    { id: 'semantic', label: 'Plain language', hint: 'Reader-first summary' },
    { id: 'methodological', label: 'Methodology', hint: 'Algorithm, gate, limits' }
  ];

  let activeRegister = $derived(
    active === 'semantic' ? registers.semantic : registers.methodological
  );
  let hiddenRegister = $derived(
    active === 'semantic' ? registers.methodological : registers.semantic
  );
</script>

<div class="progressive-semantics" {id}>
  <div class="register-toolbar">
    <SegmentedControl
      options={OPTIONS}
      value={active}
      onChange={(next) => (active = next)}
      ariaLabel="Register"
      size="sm"
    />
  </div>
  <div
    id={regionId}
    class="register-body"
    role="region"
    aria-label="{active === 'semantic' ? 'Plain-language' : 'Methodological'} register"
  >
    <p class="primary">{detail === 'short' ? activeRegister.short : activeRegister.long}</p>
  </div>
  <!-- Screen-reader-only copy of the inactive register so both remain
       addressable (Design Brief §5.7 "both registers in DOM"). -->
  <p class="sr-only">{hiddenRegister.long}</p>
</div>

<style>
  .progressive-semantics {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .register-toolbar {
    display: flex;
    align-items: center;
  }
  .register-body {
    border-left: 2px solid var(--color-accent);
    padding-left: var(--space-3);
  }
  .primary {
    margin: 0;
    color: var(--color-fg);
    font-size: var(--font-size-base);
    line-height: 1.55;
  }
  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }
</style>
