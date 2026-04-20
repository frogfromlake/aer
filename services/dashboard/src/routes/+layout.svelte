<script lang="ts">
  let { children } = $props();
</script>

<main>
  {@render children()}
</main>

<style>
  :global(html, body) {
    margin: 0;
    padding: 0;
    font-family:
      system-ui,
      -apple-system,
      'Segoe UI',
      Roboto,
      sans-serif;
    background: #0b0d10;
    color: #e6e8eb;
  }

  main {
    min-height: 100vh;
    display: grid;
    place-items: center;
  }
</style>
