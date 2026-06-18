<script lang="ts">
  // StatReadout — compact numeric readout in the instrument's mono voice
  // (Design Brief §4.1; Phase 151 design-system primitive). Used for the
  // dataset stats woven into chrome (probe / source / document counts,
  // dataset age). Value in IBM Plex Mono with tabular figures; label
  // uppercase and subdued; optional unit + trend caption + tier-tinted accent.
  //
  // Style-only via design tokens (var(--*)) — no inline colours.
  type Size = 'sm' | 'md' | 'lg';
  type Accent = 'accent' | 'validated' | 'unvalidated' | 'expired';

  interface Props {
    label?: string;
    value: string | number;
    unit?: string;
    caption?: string;
    size?: Size;
    /** Tier-tinted accent applied to the value. Omit for the default fg. */
    accent?: Accent;
  }

  let { label, value, unit, caption, size = 'md', accent }: Props = $props();

  const ACCENTS: Record<Accent, string> = {
    accent: 'var(--color-accent)',
    validated: 'var(--color-status-validated)',
    unvalidated: 'var(--color-status-unvalidated)',
    expired: 'var(--color-status-expired)'
  };
  const color = $derived(accent ? ACCENTS[accent] : undefined);
</script>

<div class="stat stat-{size}">
  {#if label}<span class="stat-label">{label}</span>{/if}
  <span class="stat-value-row">
    <span class="stat-value" style:--stat-color={color}>{value}</span>
    {#if unit}<span class="stat-unit">{unit}</span>{/if}
  </span>
  {#if caption}<span class="stat-caption">{caption}</span>{/if}
</div>

<style>
  .stat {
    display: inline-flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }
  .stat-label {
    font-family: var(--font-ui);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    white-space: nowrap;
  }
  .stat-value-row {
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
  }
  .stat-value {
    font-family: var(--font-mono);
    font-weight: var(--font-weight-semibold);
    color: var(--stat-color, var(--color-fg));
    line-height: 1.05;
    font-variant-numeric: tabular-nums;
  }
  .stat-unit {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
  }
  .stat-caption {
    font-family: var(--font-ui);
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
  }
  .stat-sm .stat-value {
    font-size: var(--font-size-lg);
  }
  .stat-md .stat-value {
    font-size: var(--font-size-2xl);
  }
  .stat-lg .stat-value {
    font-size: var(--font-size-3xl);
  }
</style>
