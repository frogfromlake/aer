<script lang="ts">
  // MethodologyBanner — Phase 122i revision (C6).
  //
  // Single visual primitive for soft methodology notes shown above Cell
  // content. Variants are tonal, not semantic: an `accent` banner says
  // "interpret with care"; a `warn` banner says "data is below the
  // recommended threshold, results may be unstable". Refusals stay in
  // `RefusalSurface`; this primitive never refuses a render, only
  // contextualises one.
  //
  // The banner is INFORMATIONAL — it does not steal focus, does not
  // block interaction, and always carries a link into the relevant
  // working paper or working note so the reader can drill into the
  // methodology.

  interface Props {
    variant?: 'accent' | 'warn';
    /** WP / working-note anchor — rendered as a trailing link. */
    anchorHref: string;
    anchorLabel: string;
  }

  let {
    variant = 'accent',
    anchorHref,
    anchorLabel,
    children
  }: Props & { children: import('svelte').Snippet } = $props();
</script>

<!-- eslint-disable svelte/no-navigation-without-resolve -- internal Reflection / methodology route -->
<aside class="methodology-banner variant-{variant}" role="note">
  <span class="banner-body">{@render children()}</span>
  <a class="banner-link" href={anchorHref} data-sveltekit-preload-data="hover">{anchorLabel}</a>
</aside>

<!-- eslint-enable svelte/no-navigation-without-resolve -->

<style>
  .methodology-banner {
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
    flex-wrap: wrap;
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-sm);
    font-size: var(--font-size-xs);
    color: var(--color-fg);
    line-height: 1.4;
    margin-bottom: var(--space-2);
  }

  .methodology-banner.variant-accent {
    background: color-mix(in srgb, var(--color-accent) 12%, var(--color-surface));
    border-left: 3px solid var(--color-accent);
  }

  .methodology-banner.variant-warn {
    background: color-mix(in srgb, var(--color-status-expired) 12%, var(--color-surface));
    border-left: 3px solid var(--color-status-expired);
  }

  .banner-body {
    flex: 1 1 0;
    min-width: 0;
  }

  .banner-link {
    color: var(--color-accent);
    text-decoration: none;
    border-bottom: 1px dotted var(--color-accent);
    flex-shrink: 0;
  }

  .banner-link:hover,
  .banner-link:focus-visible {
    color: var(--color-fg);
    border-bottom-color: var(--color-fg);
    outline: none;
  }
</style>
