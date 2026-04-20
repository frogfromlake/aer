<script lang="ts">
  import '$lib/design/global.css';
  let { children } = $props();
</script>

<a href="#main" class="skip-link">Skip to main content</a>
<main id="main">
  {@render children()}
</main>

<style>
  main {
    min-height: 100vh;
    display: grid;
    place-items: center;
    padding: var(--space-5);
  }
</style>
