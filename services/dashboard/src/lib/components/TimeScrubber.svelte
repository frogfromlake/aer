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
  import { DEFAULT_LOOKBACK_MS } from '$lib/state/url-internals';

  interface Props {
    /** Total span of the scrubber (ms). Defaults to 30 days ending "now". */
    maxSpanMs?: number;
    /** Step granularity (ms). Default 5 minutes, matching the BFF's finest resolution. */
    stepMs?: number;
    /** Minimum allowed window size (ms). Defaults to 1 hour. */
    minWindowMs?: number;
  }

  let {
    maxSpanMs = 30 * 24 * 60 * 60 * 1000,
    stepMs = 5 * 60 * 1000,
    minWindowMs = 60 * 60 * 1000
  }: Props = $props();

  // The scrubber anchors to an immutable `endReferenceMs` per mount so a
  // thumb drag does not race with the wall clock moving under the user.
  // Re-mounting the component (or a hard refresh) picks up a fresh anchor.
  let endReferenceMs = $state(Date.now());
  const windowStartMs = $derived(endReferenceMs - maxSpanMs);

  onMount(() => {
    endReferenceMs = Date.now();
  });

  const url = $derived(urlState());

  // Defaults: if the URL is empty, show the last DEFAULT_LOOKBACK_MS
  // (SSoT in url-internals.ts) so the scrubber and L1 Window agree on
  // what "reset" means.
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
    const bounded = clamp(snap(ms), windowStartMs, toMs - minWindowMs);
    setUrl({ from: new Date(bounded).toISOString() });
  }

  function setTo(ms: number): void {
    const bounded = clamp(snap(ms), fromMs + minWindowMs, endReferenceMs);
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

  const rangeSpan = $derived(endReferenceMs - windowStartMs);
  const leftPct = $derived(((fromMs - windowStartMs) / rangeSpan) * 100);
  const rightPct = $derived(((toMs - windowStartMs) / rangeSpan) * 100);

  // Wheel zoom (L2): contracts/expands the selected window around its
  // center. WheelEvent.deltaY >0 is "scroll down" → zoom out; <0 → zoom
  // in. We preventDefault so the outer page does not scroll. The
  // scrubber track is a small fixed-position element, so hijacking the
  // wheel while pointer is over it is safe and expected.
  function onWheel(e: WheelEvent) {
    if (e.ctrlKey) return; // leave pinch-zoom gestures to the browser
    e.preventDefault();
    const center = (fromMs + toMs) / 2;
    const half = (toMs - fromMs) / 2;
    const factor = e.deltaY > 0 ? 1.15 : 1 / 1.15;
    const nextHalf = Math.max(minWindowMs / 2, half * factor);
    const nextFrom = clamp(snap(center - nextHalf), windowStartMs, endReferenceMs - minWindowMs);
    const nextTo = clamp(snap(center + nextHalf), nextFrom + minWindowMs, endReferenceMs);
    setUrl({
      from: new Date(nextFrom).toISOString(),
      to: new Date(nextTo).toISOString()
    });
  }
</script>

<div class="scrubber" role="group" aria-label="Time range">
  <div class="scrubber-header">
    <div class="readout">
      <span class="label">From</span>
      <time datetime={new Date(fromMs).toISOString()}>{fmt(fromMs)}</time>
      <span class="label">To</span>
      <time datetime={new Date(toMs).toISOString()}>{fmt(toMs)}</time>
    </div>

    {#if url.from || url.to}
      <button
        class="reset-btn"
        aria-label="Reset time range"
        onclick={() => setUrl({ from: null, to: null })}
      >
        Reset
      </button>
    {/if}
  </div>

  <div
    class="track"
    role="presentation"
    style="--left: {leftPct}%; --right: {100 - rightPct}%"
    onwheel={onWheel}
  >
    <div class="track-bg"></div>
    <div class="track-highlight"></div>

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
    display: flex;
    align-items: center;
  }

  .track-bg {
    position: absolute;
    left: 0;
    right: 0;
    height: 4px;
    background: var(--color-border);
    border-radius: 2px;
    z-index: 1;
  }

  .scrubber-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
  }

  .reset-btn {
    background: transparent;
    border: 1px solid var(--color-border);
    color: var(--color-fg-muted);
    border-radius: var(--radius-sm);
    padding: 2px 8px;
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .reset-btn:hover {
    color: var(--color-fg);
    border-color: var(--color-fg-subtle);
    background: rgba(255, 255, 255, 0.05);
  }

  .track-highlight {
    position: absolute;
    left: var(--left);
    right: var(--right);
    height: 4px;
    background: #5283b8;
    border-radius: 2px;
    z-index: 2;
  }

  .thumb {
    position: absolute;
    inset: 0;
    width: 100%;
    margin: 0;
    appearance: none;
    -webkit-appearance: none;
    background: transparent;
    pointer-events: none;
    z-index: 3;
    outline: none;
  }

  .thumb::-webkit-slider-runnable-track {
    appearance: none;
    -webkit-appearance: none;
    background: transparent;
    border: none;
  }

  .thumb::-moz-range-track {
    appearance: none;
    background: transparent;
    border: none;
  }

  .thumb::-webkit-slider-thumb {
    appearance: none;
    -webkit-appearance: none;
    pointer-events: auto;
    width: 16px;
    height: 16px;
    border-radius: 50%;
    background: #fff;
    border: 2px solid #5283b8;
    cursor: grab;
    transform: translateY(-6px);
    box-shadow: 0 0 10px rgba(0, 0, 0, 0.5);
  }

  .thumb::-webkit-slider-thumb:active {
    cursor: grabbing;
    transform: translateY(-6px) scale(1.2);
    transition: transform 0.1s ease;
  }

  .thumb::-moz-range-thumb {
    appearance: none;
    pointer-events: auto;
    width: 16px;
    height: 16px;
    border-radius: 50%;
    background: #fff;
    border: 2px solid #5283b8;
    cursor: grab;
    box-shadow: 0 0 10px rgba(0, 0, 0, 0.5);
  }

  .thumb::-moz-range-thumb:active {
    cursor: grabbing;
    transform: scale(1.2);
    transition: transform 0.1s ease;
  }
</style>
