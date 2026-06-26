<script lang="ts">
  // Interactive guided tour (Slice 1 — foundation + chrome tour). A spotlight
  // overlay that walks a first-time scientist through the dashboard: it dims the
  // screen, cuts a "hole" around the element a step explains (box-shadow scrim,
  // no library), and floats an explanation card beside it. The user confirms
  // each stop with "Next" (Back / Skip also available; Esc skips).
  //
  // Mounted once in the (app) layout, so it persists across route changes — the
  // controller navigates per `step.route` (Slice 1 stays on `/`; Slices 2–3 add
  // Workbench/Reflection steps and the same controller carries the tour there).
  //
  // Targets are matched by `[data-tutorial-id]` so the tour never couples to a
  // class name; a missing target is skipped gracefully rather than stalling.
  // Step copy resolves from a `step.id`→Paraglide map; the step structure lives
  // in `$lib/state/tutorial-steps`.
  import { onMount, tick } from 'svelte';
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { m } from '$lib/paraglide/messages.js';
  import { urlState, setUrl } from '$lib/state/url.svelte';
  import { bootReady } from '$lib/state/boot.svelte';
  import { setLeaveGuardSuppressed } from '$lib/workbench/dirty.svelte';
  import {
    isTourActive,
    currentTourStep,
    tourStepIndex,
    startTour,
    nextTourStep,
    prevTourStep,
    finishTour,
    tourCompleted,
    tourReturnTo,
    clearTourReturnTo,
    welcomeSettledForTour
  } from '$lib/state/tutorial.svelte';
  import { stepCount, isLastStep } from '$lib/state/tutorial-steps';

  // Per-step title + body, keyed by step id. Kept here (not in the pure step
  // list) so the structural module stays i18n-runtime-free and node-testable.
  const COPY: Record<string, { title: () => string; body: () => string }> = {
    welcome: { title: m.tutorial_welcome_title, body: m.tutorial_welcome_body },
    surfaces: { title: m.tutorial_surfaces_title, body: m.tutorial_surfaces_body },
    dossier: { title: m.tutorial_dossier_title, body: m.tutorial_dossier_body },
    analyses: { title: m.tutorial_analyses_title, body: m.tutorial_analyses_body },
    account: { title: m.tutorial_account_title, body: m.tutorial_account_body },
    scopechip: { title: m.tutorial_scopechip_title, body: m.tutorial_scopechip_body },
    utilities: { title: m.tutorial_utilities_title, body: m.tutorial_utilities_body },
    globe: { title: m.tutorial_globe_title, body: m.tutorial_globe_body },
    scopeeditor: { title: m.tutorial_scopeeditor_title, body: m.tutorial_scopeeditor_body },
    scopegroups: { title: m.tutorial_scopegroups_title, body: m.tutorial_scopegroups_body },
    scopeapply: { title: m.tutorial_scopeapply_title, body: m.tutorial_scopeapply_body },
    pillars: { title: m.tutorial_pillars_title, body: m.tutorial_pillars_body },
    panel: { title: m.tutorial_panel_title, body: m.tutorial_panel_body },
    panelcontrols: { title: m.tutorial_panelcontrols_title, body: m.tutorial_panelcontrols_body },
    panellabel: { title: m.tutorial_panellabel_title, body: m.tutorial_panellabel_body },
    cell: { title: m.tutorial_cell_title, body: m.tutorial_cell_body },
    addpanel: { title: m.tutorial_addpanel_title, body: m.tutorial_addpanel_body },
    readingguide: { title: m.tutorial_readingguide_title, body: m.tutorial_readingguide_body },
    zen: { title: m.tutorial_zen_title, body: m.tutorial_zen_body },
    save: { title: m.tutorial_save_title, body: m.tutorial_save_body },
    reflectionnav: { title: m.tutorial_reflectionnav_title, body: m.tutorial_reflectionnav_body },
    reflection: { title: m.tutorial_reflection_title, body: m.tutorial_reflection_body },
    reflectionquestions: {
      title: m.tutorial_reflectionquestions_title,
      body: m.tutorial_reflectionquestions_body
    },
    reflectionpapers: {
      title: m.tutorial_reflectionpapers_title,
      body: m.tutorial_reflectionpapers_body
    },
    reflectioncatalogues: {
      title: m.tutorial_reflectioncatalogues_title,
      body: m.tutorial_reflectioncatalogues_body
    },
    outro: { title: m.tutorial_outro_title, body: m.tutorial_outro_body }
  };

  const SPOTLIGHT_PAD = 6;
  const CARD_GAP = 14;
  const VIEWPORT_MARGIN = 12;

  const active = $derived(isTourActive());
  const step = $derived(currentTourStep());
  const idx = $derived(tourStepIndex());
  const total = stepCount();
  const isLast = $derived(isLastStep(idx));
  const copy = $derived(step ? (COPY[step.id] ?? null) : null);
  const centered = $derived(!step || !step.targetId || step.placement === 'center');
  const progress = $derived(m.tutorial_progress({ current: idx + 1, total }));

  let targetRect = $state<DOMRect | null>(null);
  let bannerPos = $state<{ top: number; left: number } | null>(null);
  let bannerEl = $state<HTMLElement | null>(null);

  // Generation token: each step bumps it so an in-flight async resolve (route
  // nav / waiting for a target) from a previous step is discarded.
  let gen = 0;

  function queryTarget(id: string): HTMLElement | null {
    return document.querySelector<HTMLElement>(`[data-tutorial-id="${id}"]`);
  }

  // Poll (rAF) for a target that may mount after a route change; resolve null on
  // timeout or if the step changed underneath us.
  function waitForTarget(id: string, mine: number, timeoutMs = 3000): Promise<HTMLElement | null> {
    return new Promise((resolve) => {
      const started = performance.now();
      const probe = () => {
        if (gen !== mine) return resolve(null);
        const el = queryTarget(id);
        if (el) return resolve(el);
        if (performance.now() - started > timeoutMs) return resolve(null);
        requestAnimationFrame(probe);
      };
      requestAnimationFrame(probe);
    });
  }

  function computeBannerPos(rect: DOMRect, placement: string): { top: number; left: number } {
    const bw = bannerEl?.offsetWidth ?? 320;
    const bh = bannerEl?.offsetHeight ?? 160;
    const vw = window.innerWidth;
    const vh = window.innerHeight;
    let top: number;
    let left: number;
    if (placement === 'right') {
      left = rect.right + CARD_GAP;
      top = rect.top;
    } else if (placement === 'left') {
      left = rect.left - bw - CARD_GAP;
      top = rect.top;
    } else if (placement === 'top') {
      top = rect.top - bh - CARD_GAP;
      left = rect.left;
    } else {
      // bottom (default)
      top = rect.bottom + CARD_GAP;
      left = rect.left;
    }
    // Clamp into the viewport so the card is never clipped.
    left = Math.max(VIEWPORT_MARGIN, Math.min(left, vw - bw - VIEWPORT_MARGIN));
    top = Math.max(VIEWPORT_MARGIN, Math.min(top, vh - bh - VIEWPORT_MARGIN));
    return { top, left };
  }

  function reposition(): void {
    const s = currentTourStep();
    if (!s || !s.targetId || !targetRect) {
      bannerPos = null; // centred card → CSS centres it
      return;
    }
    bannerPos = computeBannerPos(targetRect, s.placement);
  }

  // Re-measure the active target on scroll/resize so the spotlight + card track it.
  function refreshRect(): void {
    const s = currentTourStep();
    if (!isTourActive() || !s || !s.targetId) return;
    const el = queryTarget(s.targetId);
    if (el) {
      targetRect = el.getBoundingClientRect();
      reposition();
    }
  }

  async function runStep(mine: number): Promise<void> {
    const s = currentTourStep();
    if (!isTourActive() || !s) {
      targetRect = null;
      bannerPos = null;
      return;
    }
    // Ensure the right surface is mounted. The Workbench has two tour modes that
    // share the `/workbench` pathname: the bare surface (ScopeEditor auto-opens)
    // and the demo-seeded split panel. We navigate when the pathname is wrong OR
    // when the demo-seed presence (`?aleph=`) does not match what this step wants
    // — a seed-presence check rather than an exact-URL compare, so the page
    // canonicalising its own query never triggers a re-navigation loop. The
    // user's real URL is restored when the tour ends (non-destructive).
    const navTarget = s.nav ?? s.route;
    const wantSeed = navTarget.includes('aleph=');
    const needsNav =
      page.url.pathname !== s.route ||
      (s.route === '/workbench' && page.url.searchParams.has('aleph') !== wantSeed);
    if (needsNav) {
      // eslint-disable-next-line svelte/no-navigation-without-resolve -- internal tour navigation
      await goto(navTarget);
      if (gen !== mine) return;
    }
    if (s.targetId) {
      const el = await waitForTarget(s.targetId, mine);
      if (gen !== mine) return;
      if (!el) {
        // Target never appeared — skip this stop rather than stall the tour.
        nextTourStep();
        return;
      }
      el.scrollIntoView({ block: 'center', inline: 'center', behavior: 'auto' });
      targetRect = el.getBoundingClientRect();
    } else {
      targetRect = null;
    }
    await tick();
    if (gen !== mine) return;
    reposition();
    bannerEl?.focus();
  }

  // Drive the tour: re-run whenever the active flag or the step index changes.
  $effect(() => {
    const isActive = isTourActive();
    const i = tourStepIndex();
    void i; // tracked so a step change re-runs
    gen += 1;
    const mine = gen;
    if (!isActive) {
      targetRect = null;
      bannerPos = null;
      return;
    }
    void runStep(mine);
  });

  function onKeydown(event: KeyboardEvent): void {
    if (!isTourActive()) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      finishTour();
      return;
    }
    if (event.key === 'Tab' && bannerEl) {
      const focusable = bannerEl.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), [tabindex]:not([tabindex="-1"])'
      );
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (!first || !last) return;
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    }
  }

  onMount(() => {
    // Consume the `?guide=open` launch trigger: start the tour and clear the
    // param so it never fights per-step route navigation. (The scope-bar Guide
    // button starts the tour directly; this also makes it deep-linkable.)
    if (urlState().guide === 'open') {
      setUrl({ guide: null });
      if (!isTourActive()) startTour();
    }
    window.addEventListener('keydown', onKeydown);
    window.addEventListener('resize', refreshRect);
    window.addEventListener('scroll', refreshRect, true);
    return () => {
      window.removeEventListener('keydown', onKeydown);
      window.removeEventListener('resize', refreshRect);
      window.removeEventListener('scroll', refreshRect, true);
    };
  });

  // Restore the user's pre-tour URL when the tour ends (undoes the seeded
  // Workbench demo so the tour is non-destructive).
  let wasActive = false;
  $effect(() => {
    const a = isTourActive();
    if (wasActive && !a) {
      const back = tourReturnTo();
      clearTourReturnTo();
      if (back && back !== page.url.pathname + page.url.search) {
        // eslint-disable-next-line svelte/no-navigation-without-resolve -- restore the user's pre-tour URL
        void goto(back).finally(() => setLeaveGuardSuppressed(false));
      } else {
        // Already where we started — lift the leave-guard suppression now.
        setLeaveGuardSuppressed(false);
      }
    }
    wasActive = a;
  });

  // First-visit auto-start: once the boot splash has cleared AND the welcome
  // greeting has played (handshake, so the two never collide), walk a brand-new
  // user (who has not completed/skipped before) through the tour. One-shot.
  let autoStarted = false;
  $effect(() => {
    const ready = bootReady() && welcomeSettledForTour();
    if (autoStarted || !ready) return;
    if (!isTourActive() && !tourCompleted() && page.url.pathname === '/') {
      autoStarted = true;
      startTour();
    }
  });
</script>

{#if active && step && copy}
  <div class="tutorial-layer">
    {#if targetRect && step.targetId}
      <!-- Click-blocker (absorbs app clicks during the tour) + visual spotlight. -->
      <div class="blocker"></div>
      <div
        class="spotlight"
        style="top:{targetRect.top - SPOTLIGHT_PAD}px; left:{targetRect.left -
          SPOTLIGHT_PAD}px; width:{targetRect.width +
          2 * SPOTLIGHT_PAD}px; height:{targetRect.height + 2 * SPOTLIGHT_PAD}px;"
      ></div>
    {:else}
      <div class="dim"></div>
    {/if}

    <!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
    <section
      class="banner"
      class:centered
      role="dialog"
      aria-modal="true"
      aria-label={m.tutorial_dialog_label()}
      tabindex="-1"
      bind:this={bannerEl}
      style={bannerPos ? `top:${bannerPos.top}px; left:${bannerPos.left}px;` : ''}
    >
      <p class="eyebrow">{progress}</p>
      <h2>{copy.title()}</h2>
      <p class="body">{copy.body()}</p>
      <div class="controls">
        <button type="button" class="skip" onclick={finishTour}>{m.tutorial_skip()}</button>
        <div class="nav">
          {#if idx > 0}
            <button type="button" class="back" onclick={prevTourStep}>{m.tutorial_back()}</button>
          {/if}
          <button type="button" class="next" onclick={nextTourStep}>
            {isLast ? m.tutorial_finish() : m.tutorial_next()}
          </button>
        </div>
      </div>
    </section>
  </div>
{/if}

<style>
  /* Scrim colour is deliberately a fixed dark wash (not theme-tinted): a light
     scrim over a light UI would not make the spotlight hole read. The hole shows
     the live, un-dimmed element through. */
  .tutorial-layer {
    position: fixed;
    inset: 0;
    z-index: 3000;
  }
  .blocker {
    position: fixed;
    inset: 0;
    z-index: 3000;
    /* Transparent: the spotlight's box-shadow paints the scrim; this only
       absorbs stray clicks to the app during the tour. */
    background: transparent;
  }
  .dim {
    position: fixed;
    inset: 0;
    z-index: 3000;
    background: rgba(6, 8, 13, 0.62);
  }
  .spotlight {
    position: fixed;
    z-index: 3001;
    border-radius: var(--radius-md);
    border: 2px solid var(--color-accent);
    box-shadow: 0 0 0 9999px rgba(6, 8, 13, 0.62);
    pointer-events: none;
    transition:
      top var(--motion-duration-base) var(--motion-ease-standard),
      left var(--motion-duration-base) var(--motion-ease-standard),
      width var(--motion-duration-base) var(--motion-ease-standard),
      height var(--motion-duration-base) var(--motion-ease-standard);
  }

  .banner {
    position: fixed;
    z-index: 3002;
    width: min(22rem, calc(100vw - 2 * var(--space-5)));
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    box-shadow: var(--elevation-3);
    padding: var(--space-4);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .banner.centered {
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
  }
  .banner:focus-visible {
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }
  .eyebrow {
    margin: 0;
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
  }
  .banner h2 {
    margin: 0;
    font-size: var(--font-size-md);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    line-height: 1.25;
  }
  .body {
    margin: 0;
    font-size: var(--font-size-sm);
    line-height: var(--line-height-base);
    color: var(--color-fg-muted);
  }
  .controls {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    margin-top: var(--space-2);
  }
  .nav {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .skip {
    appearance: none;
    background: transparent;
    border: none;
    color: var(--color-fg-subtle);
    font-family: var(--font-ui);
    font-size: var(--font-size-xs);
    cursor: pointer;
    padding: var(--space-1) var(--space-2);
  }
  .skip:hover,
  .skip:focus-visible {
    color: var(--color-fg);
    text-decoration: underline;
    outline: none;
  }
  .back {
    appearance: none;
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-fg-muted);
    font-family: var(--font-ui);
    font-size: var(--font-size-xs);
    padding: var(--space-1) var(--space-3);
    cursor: pointer;
  }
  .back:hover,
  .back:focus-visible {
    background: var(--color-surface-hover);
    color: var(--color-fg);
    outline: none;
  }
  .next {
    appearance: none;
    background: var(--color-accent);
    border: 1px solid var(--color-accent);
    border-radius: var(--radius-md);
    color: var(--color-on-accent);
    font-family: var(--font-ui);
    font-size: var(--font-size-xs);
    font-weight: var(--font-weight-medium);
    padding: var(--space-1) var(--space-4);
    cursor: pointer;
  }
  .next:hover,
  .next:focus-visible {
    filter: brightness(1.08);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }
  .back:focus-visible,
  .skip:focus-visible {
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  @media (prefers-reduced-motion: reduce) {
    .spotlight {
      transition: none;
    }
  }
</style>
