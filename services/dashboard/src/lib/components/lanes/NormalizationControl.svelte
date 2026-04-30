<script lang="ts">
  // Phase 115 — per-cell normalization toggle (LensBar header).
  //
  // Three options: Raw (Level 1), Z-score (Level 2), Percentile
  // (Level 2). The control is the user-facing operationalisation of the
  // WP-004 §6.3 dashboard implications: the default is Level 1, and any
  // jump to Level 2 attaches the Phase-115 deviation byline to the
  // active cell.
  //
  // The cross-cultural refusal surface fires server-side: when the
  // resolved scope spans more than one language and the
  // `aer_gold.metric_equivalence` registry has no deviation-or-absolute
  // entry covering both languages, the BFF returns 400 with a structured
  // RefusalPayload. The `RefusalSurface` component picks that up — this
  // control only cares about user intent, not gate state.
  import type { Normalization } from '$lib/state/url-internals';

  interface Props {
    value: Normalization;
    onChange: (next: Normalization) => void;
    /** When true, the Z-score / Percentile options render disabled with
     *  a methodological-register tooltip pointing at the resolution path. */
    baselineMissing?: boolean;
  }

  let { value, onChange, baselineMissing = false }: Props = $props();

  const OPTIONS: Array<{ id: Normalization; label: string; hint: string }> = [
    {
      id: 'raw',
      label: 'Raw',
      hint: 'Level 1 — values as stored. Always intra-culturally valid; the default for any cross-frame scope.'
    },
    {
      id: 'zscore',
      label: 'Z-score',
      hint: 'Level 2 — distance from the per-(metric, source, language) baseline in standard deviations. Requires baseline + equivalence.'
    },
    {
      id: 'percentile',
      label: 'Percentile',
      hint: 'Level 2 — within-(metric, source, language) percentile rank over the active window. Requires baseline + equivalence.'
    }
  ];

  function tooltipFor(id: Normalization): string {
    const o = OPTIONS.find((x) => x.id === id);
    if (!o) return '';
    if (id !== 'raw' && baselineMissing) {
      return `${o.hint} No baseline yet — run scripts/compute_baselines.py or wait for the next MetricBaselineExtractor sweep.`;
    }
    return o.hint;
  }
</script>

<fieldset class="normalization" aria-label="Normalization">
  <legend>Normalization</legend>
  {#each OPTIONS as opt (opt.id)}
    <label
      class:active={value === opt.id}
      class:disabled={opt.id !== 'raw' && baselineMissing}
      title={tooltipFor(opt.id)}
    >
      <input
        type="radio"
        name="normalization"
        value={opt.id}
        checked={value === opt.id}
        disabled={opt.id !== 'raw' && baselineMissing}
        onchange={() => onChange(opt.id)}
      />
      <span>{opt.label}</span>
    </label>
  {/each}
</fieldset>

<style>
  .normalization {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    border: 0;
    padding: 0;
    margin: 0;
  }
  legend {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    text-transform: uppercase;
    letter-spacing: 0.08em;
    margin-right: var(--space-2);
  }
  label {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    padding: 2px var(--space-2);
    border-radius: var(--radius-sm);
    border: 1px solid var(--color-border);
    background: var(--color-surface);
    cursor: pointer;
    font-size: var(--font-size-sm);
  }
  label.active {
    background: var(--color-accent);
    color: var(--color-bg);
    border-color: var(--color-accent);
  }
  label.disabled {
    opacity: 0.5;
    cursor: help;
  }
  input {
    position: absolute;
    opacity: 0;
    pointer-events: none;
  }
</style>
