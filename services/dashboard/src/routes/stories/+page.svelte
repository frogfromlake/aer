<script lang="ts">
  const stories = [
    { href: '/stories/button', title: 'Button', description: 'Primary, secondary, ghost.' },
    { href: '/stories/dialog', title: 'Dialog', description: 'Modal with focus trap.' },
    { href: '/stories/tooltip', title: 'Tooltip', description: 'Hover/focus supplementary text.' },
    { href: '/stories/badge', title: 'Badge', description: 'Epistemic Weight tiers.' },
    { href: '/stories/skiplink', title: 'SkipLink', description: 'Keyboard bypass to main.' }
  ];
</script>

<svelte:head>
  <title>AĒR Stories</title>
</svelte:head>

<h2>Component stories</h2>
<p class="lede">
  Route-based harness for the base component library. Each route renders one component in its
  documented variants; toggle <em>Dark</em> / <em>Light</em> in the sidebar to exercise both themes.
</p>

<ul class="cards">
  {#each stories as story (story.href)}
    <li>
      <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- internal story route -->
      <a href={story.href} data-sveltekit-preload-data="hover">
        <strong>{story.title}</strong>
        <span>{story.description}</span>
      </a>
    </li>
  {/each}
</ul>

<style>
  h2 {
    font-size: var(--font-size-2xl);
    margin: 0 0 var(--space-2);
  }
  .lede {
    color: var(--color-fg-muted);
    margin: 0 0 var(--space-6);
    max-width: 60ch;
  }
  .cards {
    list-style: none;
    padding: 0;
    margin: 0;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
    gap: var(--space-3);
  }
  .cards a {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding: var(--space-4);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    text-decoration: none;
    color: var(--color-fg);
    background: var(--color-bg-elevated);
  }
  .cards a:hover {
    border-color: var(--color-accent);
  }
  .cards strong {
    font-size: var(--font-size-base);
  }
  .cards span {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
  }
</style>
