<script lang="ts">
  // Phase 149 — "About AĒR", the re-openable home of the welcome content.
  //
  // The first-visit ambient greeting (WelcomeAmbient) is a one-time cinematic;
  // this panel is its readable, always-reachable counterpart, opened from the ⓘ
  // affordance in the Atmosphere chrome (or a deep-linked `?about=open`). It
  // introduces AĒR honestly — what it is, its purpose, who it is for, and a
  // candid statement of its state (a solo-built proof of concept, not yet
  // scientifically validated) and where it is headed.
  //
  // Pure DOM, mounted once in the (app) layout (like the Dossier overlay), so it
  // works on every surface and without the globe engine. Mirrors the Dossier
  // overlay's a11y discipline: Esc to close, focus trap, focus restore.
  import { onMount, tick } from 'svelte';
  import { urlState, setUrl } from '$lib/state/url.svelte';
  import { m } from '$lib/paraglide/messages.js';

  const url = $derived(urlState());
  const isOpen = $derived(url.about === 'open');

  function close(): void {
    setUrl({ about: null });
  }

  // ---- a11y: Esc + focus restore + Tab trap (Dossier overlay pattern) -------
  let dialogEl = $state<HTMLElement | null>(null);
  let lastFocused: HTMLElement | null = null;

  function onKeydown(e: KeyboardEvent): void {
    if (!isOpen) return;
    if (e.key === 'Escape') {
      if (e.defaultPrevented) return;
      e.preventDefault();
      close();
      return;
    }
    if (e.key === 'Tab' && dialogEl) {
      const focusable = dialogEl.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), input:not([disabled]), [tabindex]:not([tabindex="-1"])'
      );
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (!first || !last) return;
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    }
  }

  $effect(() => {
    if (isOpen) {
      if (!lastFocused) lastFocused = document.activeElement as HTMLElement | null;
      void tick().then(() => dialogEl?.focus());
    } else if (lastFocused) {
      lastFocused.focus();
      lastFocused = null;
    }
  });

  onMount(() => {
    window.addEventListener('keydown', onKeydown);
    return () => window.removeEventListener('keydown', onKeydown);
  });
</script>

{#if isOpen}
  <div class="about-backdrop" role="presentation">
    <!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
    <section
      class="about-overlay"
      role="dialog"
      aria-modal="true"
      aria-label={m.about_title()}
      tabindex="-1"
      bind:this={dialogEl}
    >
      <header class="about-header">
        <div class="about-titles">
          <p class="eyebrow">{m.about_eyebrow()}</p>
          <h2>{m.about_title()}</h2>
        </div>
        <button type="button" class="close-btn" onclick={close} aria-label={m.about_close()}
          >×</button
        >
      </header>

      <p class="about-lede">{m.about_lede()}</p>

      <div class="about-sections">
        <section class="about-block">
          <h3>{m.about_what_title()}</h3>
          <p>{m.about_what_body()}</p>
        </section>
        <!-- The methodological signature (WP-001 function-over-form · WP-004
             equivalence registry / juxtaposition · WP-003/007 negative space) —
             the rigor showcase for a research audience. -->
        <section class="about-block">
          <h3>{m.about_method_title()}</h3>
          <p>{m.about_method_body()}</p>
        </section>
        <section class="about-block">
          <h3>{m.about_purpose_title()}</h3>
          <p>{m.about_purpose_body()}</p>
        </section>
        <section class="about-block">
          <h3>{m.about_who_title()}</h3>
          <p>{m.about_who_body()}</p>
        </section>
        <!-- The value proposition for research institutions: what a researcher
             can concretely DO with AĒR (and why it survives peer review). -->
        <section class="about-block about-power">
          <h3>{m.about_power_title()}</h3>
          <p>{m.about_power_body()}</p>
        </section>
        <!-- The honest, curated state-of-the-project note: proof of concept,
             solo-built, not yet validated. Marked, not hidden. -->
        <section class="about-block about-state">
          <h3>{m.about_state_title()}</h3>
          <p>{m.about_state_body()}</p>
        </section>
        <section class="about-block">
          <h3>{m.about_future_title()}</h3>
          <p>{m.about_future_body()}</p>
        </section>
      </div>

      <footer class="about-footer">
        <span class="about-footer-label">{m.about_links_title()}</span>
        <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- internal Reflection route -->
        <a class="about-link" href="/reflection" onclick={close}>{m.about_link_reflection()}</a>
      </footer>
    </section>
  </div>
{/if}

<style>
  .about-backdrop {
    position: fixed;
    /* Start after the fixed SideRail so the panel centres in the visible area. */
    inset: 0 0 0 var(--rail-width, 184px);
    background: color-mix(in srgb, var(--color-bg) 70%, transparent);
    backdrop-filter: blur(3px);
    -webkit-backdrop-filter: blur(3px);
    z-index: 40;
    display: grid;
    place-items: center;
    padding: var(--space-5);
    overflow-y: auto;
  }

  .about-overlay {
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    width: min(52rem, 92%);
    max-height: 90vh;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    /* No top padding — the sticky header supplies the top inset itself. Padding
       the top here AND negative-margining the header into it double-counted with
       the flex gap and pulled the first content (lede) up under the header. */
    padding: 0 var(--space-6) var(--space-6);
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.4);
  }

  .about-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--space-3);
    border-bottom: 1px solid var(--color-border);
    /* Keep the title + × reachable while the panel scrolls. `.about-overlay` is
       the scroll container (no top padding); the header bleeds to the left/right
       edges (negative side margins over the overlay's side padding) and supplies
       the top inset via its own padding, so scrolled content never peeks around
       it and the top corners stay rounded. */
    position: sticky;
    top: 0;
    z-index: 3;
    background: var(--color-bg-elevated);
    margin: 0 calc(-1 * var(--space-6));
    padding: var(--space-6) var(--space-6) var(--space-4);
    border-top-left-radius: var(--radius-md);
    border-top-right-radius: var(--radius-md);
  }
  .about-titles .eyebrow {
    margin: 0 0 var(--space-1);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.12em;
    color: var(--color-fg-subtle);
    font-family: var(--font-mono);
  }
  .about-titles h2 {
    margin: 0;
    font-size: var(--font-size-lg);
    color: var(--color-fg);
    line-height: 1.35;
  }
  .close-btn {
    flex: none;
    width: 2rem;
    height: 2rem;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--color-fg-muted);
    font-size: 1.25rem;
    line-height: 1;
    cursor: pointer;
  }
  .close-btn:hover,
  .close-btn:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-accent);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .about-lede {
    margin: 0;
    font-size: var(--font-size-md, 1rem);
    line-height: 1.6;
    color: var(--color-fg);
  }

  .about-sections {
    display: grid;
    gap: var(--space-5);
  }
  .about-block h3 {
    margin: 0 0 var(--space-2);
    font-size: var(--font-size-sm);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-accent);
  }
  .about-block p {
    margin: 0;
    font-size: var(--font-size-sm);
    line-height: 1.6;
    color: var(--color-fg-muted);
  }
  /* The research value proposition — a quiet accent rule marks it as the
     inviting "what's in it for you" block. */
  .about-power {
    border-left: 2px solid var(--color-accent);
    padding-left: var(--space-4);
  }
  /* The state note is the candid disclosure — give it a quiet methodological
     frame (a left rule), never an alarming colour. */
  .about-state {
    border-left: 2px solid var(--color-border-strong, var(--color-border));
    padding-left: var(--space-4);
  }

  .about-footer {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    border-top: 1px solid var(--color-border);
    padding-top: var(--space-4);
  }
  .about-footer-label {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    font-family: var(--font-mono);
  }
  .about-link {
    font-size: var(--font-size-sm);
    color: var(--color-accent);
    text-decoration: none;
  }
  .about-link:hover,
  .about-link:focus-visible {
    text-decoration: underline;
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }
</style>
