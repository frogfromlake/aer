<script lang="ts">
  // Phase 149 — first-visit ambient greeting over the globe.
  //
  // A NON-BLOCKING cinematic: the AĒR wordmark with a tagline that cross-fades
  // to a second line, then the whole layer fades away. It greets and orients
  // without nagging — it plays ONCE PER SESSION (a sessionStorage flag, so it
  // does NOT replay on reload, but a fresh tab / sign-in shows it again), the
  // globe stays live and rotatable underneath (the layer is pointer-events:none,
  // only the Skip button is interactive), and any click / key / wheel dismisses
  // it instantly. Its readable counterpart lives forever behind the ⓘ "About
  // AĒR" affordance, so nothing is lost by skipping.
  //
  // It waits for `bootReady()` so it never competes with the boot splash, and it
  // respects `prefers-reduced-motion` (no cross-fade — the lines are shown
  // briefly, statically, then removed).
  import { onMount } from 'svelte';
  import { bootReady } from '$lib/state/boot.svelte';
  import { settleWelcomeForTour } from '$lib/state/tutorial.svelte';
  import { m } from '$lib/paraglide/messages.js';

  // sessionStorage (not localStorage): the greeting survives a reload within the
  // same browsing session but plays again for a new session / sign-in.
  const STORAGE_KEY = 'aer.welcome_seen';
  const LINE_MS = 3200; // dwell on line 1 before crossfading to line 2
  const TOTAL_MS = 6400; // when the whole layer begins its exit fade
  const FADE_MS = 700; // exit-fade duration before unmount

  let showing = $state(false);
  let fadingOut = $state(false);
  let step = $state(0); // 0 = tagline, 1 = "external perspective" line
  let started = false;
  const timers: ReturnType<typeof setTimeout>[] = [];

  const reducedMotion = (): boolean =>
    typeof window !== 'undefined' &&
    window.matchMedia?.('(prefers-reduced-motion: reduce)').matches === true;

  function alreadySeen(): boolean {
    try {
      return sessionStorage.getItem(STORAGE_KEY) === '1';
    } catch {
      // Private mode / blocked storage → treat as seen so we never loop the
      // greeting on every load.
      return true;
    }
  }

  function markSeen(): void {
    try {
      sessionStorage.setItem(STORAGE_KEY, '1');
    } catch {
      /* best-effort; nothing to do if storage is unavailable */
    }
  }

  function clearTimers(): void {
    for (const t of timers) clearTimeout(t);
    timers.length = 0;
  }

  function finish(): void {
    if (!showing || fadingOut) return;
    clearTimers();
    fadingOut = true;
    markSeen();
    timers.push(
      setTimeout(() => {
        showing = false;
        // Hand off to the guided tour only now — the greeting has fully faded
        // out and unmounted, so a first-visit auto-start cannot overlap this
        // layer. (Calling this at the START of the fade let the tour card appear
        // during the 700 ms exit, overlaying the still-visible greeting.)
        settleWelcomeForTour();
      }, FADE_MS)
    );
  }

  function start(): void {
    if (started || alreadySeen()) return;
    started = true;
    showing = true;
    if (reducedMotion()) {
      // No motion: show line 1 statically, hold a touch longer, then exit.
      timers.push(setTimeout(finish, TOTAL_MS));
      return;
    }
    timers.push(setTimeout(() => (step = 1), LINE_MS));
    timers.push(setTimeout(finish, TOTAL_MS));
  }

  const lineText = $derived(step === 0 ? m.welcome_ambient_line1() : m.welcome_ambient_line2());

  // Begin only once the boot splash has cleared (globe interactive). The effect
  // re-checks on readiness changes; `started` makes it one-shot.
  $effect(() => {
    if (!bootReady()) return;
    if (alreadySeen()) {
      // Greeting won't play this session → open the tour's auto-start gate now.
      settleWelcomeForTour();
    } else {
      start();
    }
  });

  onMount(() => {
    // Any deliberate interaction wipes the greeting away immediately.
    const dismiss = () => finish();
    window.addEventListener('pointerdown', dismiss);
    window.addEventListener('keydown', dismiss);
    window.addEventListener('wheel', dismiss, { passive: true });
    return () => {
      clearTimers();
      window.removeEventListener('pointerdown', dismiss);
      window.removeEventListener('keydown', dismiss);
      window.removeEventListener('wheel', dismiss);
    };
  });
</script>

{#if showing}
  <div class="welcome-ambient" class:out={fadingOut}>
    <div class="wa-inner">
      <span class="wa-mark" aria-hidden="true">AĒR</span>
      <!-- Key on `step` so the line element is recreated → its enter animation
           replays, giving a per-line cross-fade. -->
      {#key step}
        <span class="wa-line" aria-hidden="true">{lineText}</span>
      {/key}
    </div>
    <button type="button" class="wa-skip" onclick={finish}>{m.welcome_ambient_skip()}</button>
  </div>
{/if}

<style>
  .welcome-ambient {
    position: fixed;
    inset: 0;
    z-index: 400;
    display: grid;
    place-items: center;
    /* Click-through: the globe stays fully interactive underneath. Only the
       Skip button re-enables pointer events. */
    pointer-events: none;
    /* A whisper of darkening at centre so the text reads over a busy globe,
       fading to fully transparent at the edges. */
    background: radial-gradient(
      ellipse at center,
      color-mix(in srgb, var(--color-bg) 38%, transparent),
      transparent 62%
    );
    opacity: 1;
    animation: wa-enter 900ms ease-out;
    transition: opacity 700ms ease-in-out;
  }
  .welcome-ambient.out {
    opacity: 0;
  }

  .wa-inner {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-3);
    text-align: center;
    padding: 0 var(--space-5);
  }
  .wa-mark {
    font-family: var(--font-mono);
    font-size: clamp(2rem, 6vw, 3.5rem);
    letter-spacing: 0.2em;
    color: var(--color-fg);
    text-shadow: 0 2px 24px rgba(0, 0, 0, 0.6);
  }
  .wa-line {
    font-size: clamp(0.95rem, 2.2vw, 1.25rem);
    line-height: 1.5;
    max-width: 32rem;
    color: var(--color-fg-muted);
    text-shadow: 0 1px 16px rgba(0, 0, 0, 0.7);
    /* Re-key the fade on every step change so the line cross-fades in. */
    animation: wa-line-in 700ms ease-out;
  }

  .wa-skip {
    pointer-events: auto;
    position: absolute;
    /* Centred along the bottom edge so it never overlaps the corner chrome
       (theme/account controls bottom-right, SideRail bottom-left). */
    left: 50%;
    transform: translateX(-50%);
    bottom: var(--space-6);
    padding: var(--space-1) var(--space-3);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    background: color-mix(in srgb, var(--color-bg-elevated) 60%, transparent);
    color: var(--color-fg-muted);
    font-size: var(--font-size-xs);
    cursor: pointer;
    backdrop-filter: blur(4px);
    -webkit-backdrop-filter: blur(4px);
  }
  .wa-skip:hover,
  .wa-skip:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-accent);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  @keyframes wa-enter {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }
  @keyframes wa-line-in {
    from {
      opacity: 0;
      transform: translateY(4px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .welcome-ambient {
      animation: none;
    }
    .wa-line {
      animation: none;
    }
  }
</style>
