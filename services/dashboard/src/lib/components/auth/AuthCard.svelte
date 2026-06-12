<script lang="ts">
  // Shared shell for the pre-auth pages (login / accept-invite / forgot /
  // reset). Centres a single card on the dark atmospheric AĒR backdrop, with
  // the wordmark, a title, and slots for the form and a footer.
  import type { Snippet } from 'svelte';

  interface Props {
    title: string;
    subtitle?: string;
    children: Snippet;
    footer?: Snippet;
  }

  let { title, subtitle, children, footer }: Props = $props();
</script>

<main class="auth-page">
  <div class="atmosphere" aria-hidden="true"></div>

  <section class="card" aria-labelledby="auth-title">
    <header class="brand">
      <span class="wordmark">AĒR</span>
      <span class="tagline">Atmospheric sensor for human discourse</span>
    </header>

    <h1 id="auth-title" class="title">{title}</h1>
    {#if subtitle}
      <p class="subtitle">{subtitle}</p>
    {/if}

    <div class="body">
      {@render children()}
    </div>

    {#if footer}
      <footer class="card-footer">
        {@render footer()}
      </footer>
    {/if}
  </section>
</main>

<style>
  .auth-page {
    position: fixed;
    inset: 0;
    display: grid;
    place-items: center;
    padding: var(--space-5);
    background: var(--color-bg);
    overflow: auto;
  }

  /* Faint cyan "atmosphere" glow — evokes the AĒR air/atmosphere identity
     without competing with the form. */
  .atmosphere {
    position: absolute;
    inset: 0;
    pointer-events: none;
    background:
      radial-gradient(
        60rem 40rem at 50% -10%,
        color-mix(in oklab, var(--color-accent) 12%, transparent),
        transparent 70%
      ),
      radial-gradient(
        40rem 30rem at 85% 110%,
        color-mix(in oklab, var(--color-viridis-25) 14%, transparent),
        transparent 70%
      );
  }

  .card {
    position: relative;
    width: min(100%, 25rem);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    box-shadow: var(--elevation-3);
    padding: var(--space-6) var(--space-6) var(--space-5);
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .brand {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    align-items: center;
    text-align: center;
    margin-bottom: var(--space-2);
  }
  .wordmark {
    font-family: var(--font-ui);
    font-weight: var(--font-weight-semibold);
    font-size: var(--font-size-2xl);
    letter-spacing: var(--letter-spacing-wide);
    color: var(--color-fg);
  }
  .tagline {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    letter-spacing: var(--letter-spacing-wide);
    text-transform: uppercase;
  }

  /* Left-aligned section heading so it reads as a form title, not a second
     centred wordmark. A hairline rule separates the brand block from the form. */
  .title {
    font-family: var(--font-ui);
    font-size: var(--font-size-md);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    margin: 0;
    padding-top: var(--space-4);
    border-top: 1px solid var(--color-border);
    text-align: left;
  }
  .subtitle {
    margin: calc(-1 * var(--space-2)) 0 0;
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    text-align: left;
    line-height: var(--line-height-base);
  }

  .body {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .card-footer {
    margin-top: var(--space-1);
    padding-top: var(--space-4);
    border-top: 1px solid var(--color-border);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    align-items: center;
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
  }

  @media (prefers-reduced-motion: reduce) {
    .atmosphere {
      background: none;
    }
  }
</style>
