<script lang="ts">
  // ADR-038 — one compact, consistent empty-state for cells. After Tier-2
  // drop+note, a lacking source no longer renders an empty cell at all, so this
  // covers only the genuinely-empty cases that remain (a merged union with no
  // rows; a single source with nothing in the window). The "why" nuance
  // (structural Negative Space vs no captured article in the window) lives in the
  // cell's "how to read" + the per-source metadata-coverage matrix — not a wall
  // of text here, so sibling cells read consistently.
  let { label }: { label?: string } = $props();
</script>

<p class="cell-empty" role="note">
  {#if label}
    No <code>{label}</code> values in this window and scope.
  {:else}
    No data in this window and scope.
  {/if}
  Nothing to chart — never a coerced zero. See “how to read”.
</p>

<style>
  .cell-empty {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
    line-height: var(--line-height-loose);
  }
  .cell-empty code {
    font-family: var(--font-mono);
    color: var(--color-fg);
  }
</style>
