<script lang="ts">
  import { page } from '$app/state';
  import type { Snippet } from 'svelte';

  interface Props {
    children?: Snippet;
  }

  let { children }: Props = $props();

  const stories = [
    { href: '/stories/button', title: 'Button' },
    { href: '/stories/dialog', title: 'Dialog' },
    { href: '/stories/tooltip', title: 'Tooltip' },
    { href: '/stories/badge', title: 'Badge' },
    { href: '/stories/skiplink', title: 'SkipLink' }
  ];

  let theme = $state<'dark' | 'light'>('dark');

  $effect(() => {
    document.documentElement.setAttribute('data-theme', theme);
    return () => document.documentElement.setAttribute('data-theme', 'dark');
  });
</script>

<svelte:head>
  <title>AĒR Stories — {page.url.pathname}</title>
</svelte:head>

<div class="shell">
  <aside>
    <h1>AĒR Stories</h1>
    <nav aria-label="Component stories">
      <ul>
        <!-- eslint-disable svelte/no-navigation-without-resolve -- internal story routes -->
        {#each stories as story (story.href)}
          <li>
            <a
              href={story.href}
              class:active={page.url.pathname === story.href}
              data-sveltekit-preload-data="hover">{story.title}</a
            >
          </li>
        {/each}
        <!-- eslint-enable svelte/no-navigation-without-resolve -->
      </ul>
    </nav>
    <div class="theme-toggle">
      <span id="theme-label">Theme</span>
      <div role="radiogroup" aria-labelledby="theme-label">
        <button
          type="button"
          role="radio"
          aria-checked={theme === 'dark'}
          onclick={() => (theme = 'dark')}
          class:pressed={theme === 'dark'}>Dark</button
        >
        <button
          type="button"
          role="radio"
          aria-checked={theme === 'light'}
          onclick={() => (theme = 'light')}
          class:pressed={theme === 'light'}>Light</button
        >
      </div>
    </div>
  </aside>
  <main id="main">
    {#if children}{@render children()}{/if}
  </main>
</div>

<style>
  .shell {
    display: grid;
    grid-template-columns: 240px 1fr;
    min-height: 100vh;
  }

  aside {
    border-right: 1px solid var(--color-border);
    padding: var(--space-5);
    background: var(--color-bg-elevated);
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
  }

  h1 {
    font-size: var(--font-size-lg);
    margin: 0;
  }

  nav ul {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  nav a {
    display: block;
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-md);
    color: var(--color-fg-muted);
    text-decoration: none;
    font-size: var(--font-size-sm);
  }

  nav a:hover {
    background: var(--color-surface-hover);
    color: var(--color-fg);
  }

  nav a.active {
    background: var(--color-surface);
    color: var(--color-fg);
  }

  main {
    padding: var(--space-6);
    overflow: auto;
  }

  .theme-toggle {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .theme-toggle span {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    text-transform: uppercase;
    letter-spacing: var(--letter-spacing-wide);
  }

  .theme-toggle div {
    display: inline-flex;
    gap: var(--space-1);
  }

  .theme-toggle button {
    padding: var(--space-1) var(--space-3);
    font-size: var(--font-size-sm);
    background: transparent;
    color: var(--color-fg-muted);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    cursor: pointer;
  }

  .theme-toggle button.pressed {
    background: var(--color-surface);
    color: var(--color-fg);
    border-color: var(--color-accent);
  }
</style>
