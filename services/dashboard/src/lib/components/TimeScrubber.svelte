<script lang="ts">
  // Horizontal time-range scrubber (ROADMAP Phase 99b).
  //
  // Two thumbs over a fixed maximum window (default: last 30 days).
  // Keyboard model:
  //   ArrowLeft/Right: ±1 step (minutes configurable by resolution).
  //   Shift+ArrowLeft/Right: ±10 steps.
  //   Home/End: jump thumb to window start/end.
  //   Each thumb is independently focusable.
  //
  // All state is URL-backed via `$lib/state/url.svelte.ts` — scrubbing is
  // the same primitive a deep link uses. The component never owns its
  // own time range, only reads from and writes to the URL store.
  import { onMount } from 'svelte';
  import { setUrl, urlState } from '$lib/state/url.svelte';

  interface Props {
    /** Total span of the scrubber (ms). Defaults to 30 days ending "now". */
    maxSpanMs?: number;
    /** Step granularity (ms). Default 5 minutes, matching the BFF's finest resolution. */
    stepMs?: number;
  }

  let { maxSpanMs = 30 * 24 * 60 * 60 * 1000, stepMs = 5 * 60 * 1000 }: Props = $props();

  // The scrubber anchors to an immutable `endReferenceMs` per mount so a
  // thumb drag does not race with the wall clock moving under the user.
  // Re-mounting the component (or a hard refresh) picks up a fresh anchor.
  let endReferenceMs = $state(Date.now());
  const windowStartMs = $derived(endReferenceMs - maxSpanMs);

  onMount(() => {
    endReferenceMs = Date.now();
  });

  const url = $derived(urlState());

  // Defaults: if the URL is empty, show the last 7 days. This keeps the
  // first render non-jarring and matches the ROADMAP's 24h activity
  // window by straddling it.
  const DEFAULT_LOOKBACK_MS = 7 * 24 * 60 * 60 * 1000;
  const fromMs = $derived.by<number>(() => {
    const parsed = url.from ? Date.parse(url.from) : NaN;
    return Number.isFinite(parsed) ? parsed : endReferenceMs - DEFAULT_LOOKBACK_MS;
  });
  const toMs = $derived.by<number>(() => {
    const parsed = url.to ? Date.parse(url.to) : NaN;
    return Number.isFinite(parsed) ? parsed : endReferenceMs;
  });

  function clamp(v: number, lo: number, hi: number): number {
    return Math.max(lo, Math.min(hi, v));
  }

  function snap(ms: number): number {
    // Round to the nearest step so keyboard output is always aligned.
    return Math.round(ms / stepMs) * stepMs;
  }

  function setFrom(ms: number): void {
    const bounded = clamp(snap(ms), windowStartMs, toMs - stepMs);
    setUrl({ from: new Date(bounded).toISOString() });
  }

  function setTo(ms: number): void {
    const bounded = clamp(snap(ms), fromMs + stepMs, endReferenceMs);
    setUrl({ to: new Date(bounded).toISOString() });
  }

  function nudge(kind: 'from' | 'to', direction: -1 | 1, big: boolean) {
    const delta = stepMs * direction * (big ? 10 : 1);
    if (kind === 'from') setFrom(fromMs + delta);
    else setTo(toMs + delta);
  }

  function onKeydown(kind: 'from' | 'to') {
    return (e: KeyboardEvent) => {
      switch (e.key) {
        case 'ArrowLeft':
          e.preventDefault();
          nudge(kind, -1, e.shiftKey);
          break;
        case 'ArrowRight':
          e.preventDefault();
          nudge(kind, +1, e.shiftKey);
          break;
        case 'Home':
          e.preventDefault();
          if (kind === 'from') setFrom(windowStartMs);
          else setTo(fromMs + stepMs);
          break;
        case 'End':
          e.preventDefault();
          if (kind === 'from') setFrom(toMs - stepMs);
          else setTo(endReferenceMs);
          break;
      }
    };
  }

  function fmt(ms: number): string {
    const d = new Date(ms);
    return d.toISOString().slice(0, 16).replace('T', ' ') + 'Z';
  }
</script>

<div class="scrubber" role="group" aria-label="Time range">
  <div class="readout">
    <span class="label">From</span>
    <time datetime={new Date(fromMs).toISOString()}>{fmt(fromMs)}</time>
    <span class="label">To</span>
    <time datetime={new Date(toMs).toISOString()}>{fmt(toMs)}</time>
  </div>

  <div class="track" role="presentation">
    <input
      type="range"
      class="thumb from"
      aria-label="Range start"
      min={windowStartMs}
      max={endReferenceMs}
      step={stepMs}
      value={fromMs}
      oninput={(e) => setFrom(Number((e.currentTarget as HTMLInputElement).value))}
      onkeydown={onKeydown('from')}
    />
    <input
      type="range"
      class="thumb to"
      aria-label="Range end"
      min={windowStartMs}
      max={endReferenceMs}
      step={stepMs}
      value={toMs}
      oninput={(e) => setTo(Number((e.currentTarget as HTMLInputElement).value))}
      onkeydown={onKeydown('to')}
    />
  </div>
</div>

<style>
  .scrubber {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    background: rgba(0, 0, 0, 0.6);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    backdrop-filter: blur(8px);
    color: var(--color-fg);
    font-size: var(--font-size-xs);
  }
  .readout {
    display: grid;
    grid-template-columns: auto 1fr auto 1fr;
    gap: var(--space-2) var(--space-3);
    font-family: var(--font-family-mono);
    color: var(--color-fg-muted);
  }
  .readout .label {
    color: var(--color-fg-subtle);
  }
  time {
    color: var(--color-fg);
  }
  .track {
    position: relative;
    height: 1.5rem;
  }
  .thumb {
    position: absolute;
    inset: 0;
    width: 100%;
    margin: 0;
    background: transparent;
    pointer-events: auto;
    /* Stack the two native sliders so their thumbs overlap the same bar. */
  }
  .thumb::-webkit-slider-runnable-track {
    height: 2px;
    background: var(--color-border);
  }
  .thumb::-moz-range-track {
    height: 2px;
    background: var(--color-border);
  }
</style>
