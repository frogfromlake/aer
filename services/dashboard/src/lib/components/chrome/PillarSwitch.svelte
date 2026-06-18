<script lang="ts">
  // PillarSwitch — Phase 122h / ADR-033 §2.
  //
  // Three equally-prominent tiles at the top of the Workbench. Each tile
  // carries the Pillar's German question (`Was ist jetzt da?` /
  // `Wie hat es sich verschoben?` / `Wie hängt es zusammen?`), the abbr,
  // glyph, and accent colour. The active tile is filled with the pillar
  // colour mix; inactive tiles are muted but readable. The active tile
  // also surfaces a one-line plain-language explanation below — most
  // people do not know what Aleph / Episteme / Rhizome mean; the UI
  // teaches them passively (ADR-033 §2 last paragraph).
  //
  // Keyboard shortcuts (window-level when this component is mounted):
  //   1 → Aleph, 2 → Episteme, 3 → Rhizome.
  //
  // Click switches via `$lib/pillar.pickPillar()` which reconciles the
  // URL viewMode so deep-linked Cell-views recover correctly.
  import { onMount } from 'svelte';
  import { urlState } from '$lib/state/url.svelte';
  import { PILLAR_DEFINITIONS, getPillar } from '$lib/presentations';
  import { pickPillar } from '$lib/pillar';
  import { m } from '$lib/paraglide/messages.js';
  import type { PillarId } from '$lib/state/url-internals';

  // Phase 144 — the plain-language pillar question + stance copy moved to
  // Paraglide chrome messages (was PILLAR_QUESTIONS / PILLAR_PLAIN_LANGUAGE in
  // pillar-internals.ts). Per-id message-function maps keep the template's
  // dynamic `p.id` lookup while staying reactive to the locale switch.
  const PILLAR_QUESTION: Record<PillarId, () => string> = {
    aleph: m.chrome_pillar_question_aleph,
    episteme: m.chrome_pillar_question_episteme,
    rhizome: m.chrome_pillar_question_rhizome
  };
  const PILLAR_PLAIN: Record<PillarId, () => string> = {
    aleph: m.chrome_pillar_plain_aleph,
    episteme: m.chrome_pillar_plain_episteme,
    rhizome: m.chrome_pillar_plain_rhizome
  };

  const url = $derived(urlState());
  // Phase 122i revision (A5): match the Workbench-page priority order —
  // pillar-state URLs put the active pillar in `?activePillar=`, legacy
  // flat URLs in `?viewingMode=`. Reading only `viewingMode` (Phase-122h
  // behaviour) made the tiles permanently show Aleph as active under
  // pillar-state URLs.
  const activeId = $derived<PillarId>(url.activePillar ?? 'aleph');
  const activeDef = $derived(getPillar(activeId));

  // Keyboard shortcuts. Only active when no input/textarea has focus —
  // otherwise typing "1" in a form field would jump pillars.
  onMount(() => {
    function onKeyDown(e: KeyboardEvent) {
      if (e.target instanceof HTMLInputElement) return;
      if (e.target instanceof HTMLTextAreaElement) return;
      if (e.target instanceof HTMLSelectElement) return;
      if (e.metaKey || e.ctrlKey || e.altKey) return;
      if (e.key === '1') pickPillar('aleph');
      else if (e.key === '2') pickPillar('episteme');
      else if (e.key === '3') pickPillar('rhizome');
    }
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  });
</script>

<section class="pillar-switch" aria-label={m.chrome_pillar_switch_aria()}>
  <div class="tiles" role="radiogroup" aria-label={m.chrome_pillar_radiogroup_aria()}>
    {#each PILLAR_DEFINITIONS as p (p.id)}
      {@const isActive = p.id === activeId}
      <button
        type="button"
        role="radio"
        aria-checked={isActive}
        class="tile tile-{p.id}"
        class:active={isActive}
        style:--pillar-color={p.color}
        title="{p.label} — {p.blurb}"
        onclick={() => pickPillar(p.id)}
      >
        <span class="tile-head">
          <span class="tile-glyph" aria-hidden="true">{p.glyph}</span>
          <span class="tile-name">{p.label}</span>
          <span class="tile-key" aria-hidden="true"
            >{p.id === 'aleph' ? '1' : p.id === 'episteme' ? '2' : '3'}</span
          >
        </span>
        <span class="tile-question">{PILLAR_QUESTION[p.id]()}</span>
      </button>
    {/each}
  </div>

  <p class="active-description">
    <span class="active-eyebrow">{activeDef.label}</span>
    {PILLAR_PLAIN[activeId]()}
    <span class="active-meta" title={activeDef.description}>ⓘ</span>
  </p>
</section>

<style>
  .pillar-switch {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .tiles {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: var(--space-3);
  }

  .tile {
    appearance: none;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: var(--space-3) var(--space-4);
    color: var(--color-fg-muted);
    cursor: pointer;
    text-align: left;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    transition:
      background var(--motion-duration-fast) var(--motion-ease-standard),
      border-color var(--motion-duration-fast) var(--motion-ease-standard),
      color var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .tile:hover,
  .tile:focus-visible {
    color: var(--color-fg);
    border-color: color-mix(in srgb, var(--pillar-color) 50%, var(--color-border));
    background: color-mix(in srgb, var(--pillar-color) 6%, var(--color-surface));
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .tile.active {
    background: color-mix(in srgb, var(--pillar-color) 18%, var(--color-surface));
    border-color: var(--pillar-color);
    color: var(--color-fg);
    box-shadow: inset 0 -3px 0 var(--pillar-color);
  }

  .tile-head {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .tile-glyph {
    font-size: 1.4rem;
    line-height: 1;
    color: var(--pillar-color);
  }

  .tile-name {
    font-family: var(--font-mono);
    font-size: var(--font-size-md);
    font-weight: var(--font-weight-semibold);
    color: var(--pillar-color);
    letter-spacing: 0.02em;
  }

  .tile-key {
    margin-left: auto;
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--color-fg-subtle);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 1px 5px;
    line-height: 1;
  }

  .tile.active .tile-key {
    color: var(--pillar-color);
    border-color: var(--pillar-color);
  }

  .tile-question {
    font-size: var(--font-size-sm);
    color: var(--color-fg);
    font-style: italic;
    line-height: 1.35;
  }

  .active-description {
    margin: 0;
    padding: 0 var(--space-2);
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
    flex-wrap: wrap;
  }

  .active-eyebrow {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    flex-shrink: 0;
  }

  .active-meta {
    color: var(--color-fg-subtle);
    cursor: help;
  }

  @media (max-width: 800px) {
    .tiles {
      grid-template-columns: 1fr;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .tile {
      transition: none;
    }
  }
</style>
