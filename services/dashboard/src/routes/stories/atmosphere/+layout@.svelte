<script lang="ts">
  // Layout reset (`@`): skip the stories sidebar so atmosphere stories can render
  // full-bleed. Visual snapshots and a11y scans must see the canvas alone.
  import type { Snippet } from 'svelte';

  interface Props {
    children?: Snippet;
  }

  let { children }: Props = $props();
</script>

<div class="full-bleed">
  {#if children}{@render children()}{/if}
</div>

<style>
  .full-bleed {
    position: fixed;
    inset: 0;
    background: #000;
  }
</style>
