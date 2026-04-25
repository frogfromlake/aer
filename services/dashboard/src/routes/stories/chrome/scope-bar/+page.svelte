<script lang="ts">
  import ScopeBar from '$lib/components/chrome/ScopeBar.svelte';

  let demoText = $state('Surface I — Atmosphere');
</script>

<div class="story">
  <h2>ScopeBar</h2>
  <p class="desc">
    Slot-based top navigation strip (Design Brief §3.2). Accepts per-surface content: time window +
    resolution on Surface I, lane switcher on Surface II, section anchor on Surface III.
    Fixed-position in production; shown inline here for visual review.
  </p>

  <h3>Default slot</h3>
  <div class="preview-bar">
    <ScopeBar label="Demo scope bar">
      <span class="demo-content">{demoText}</span>
      <input
        type="text"
        bind:value={demoText}
        placeholder="Type scope bar content…"
        class="demo-input"
        aria-label="Edit demo content"
      />
    </ScopeBar>
  </div>

  <h3>Surface I content (example)</h3>
  <div class="preview-bar">
    <ScopeBar label="Atmosphere surface controls">
      <span class="window-label">2026-04-18 00:00Z → 2026-04-25 00:00Z</span>
      <label class="resolution-label">
        <span class="label-text">Resolution</span>
        <select aria-label="Temporal resolution">
          <option>5min</option>
          <option selected>hourly</option>
          <option>daily</option>
        </select>
      </label>
    </ScopeBar>
  </div>

  <div class="notes">
    <h3>A11y</h3>
    <ul>
      <li>
        div[role="navigation"][aria-label] — label is a required prop (default: "Surface
        navigation")
      </li>
      <li>Inner content is fully slot-defined — each surface controls its own tab stops</li>
      <li>
        Right edge follows --tray-right-edge CSS var (updated by MethodologyTray in push-mode)
      </li>
    </ul>
  </div>
</div>

<style>
  .story {
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
  }
  h2 {
    font-size: var(--font-size-xl);
    margin: 0;
  }
  h3 {
    font-size: var(--font-size-base);
    margin: 0 0 var(--space-1);
  }
  .desc {
    color: var(--color-fg-muted);
    font-size: var(--font-size-sm);
    line-height: var(--line-height-loose);
    margin: 0;
    max-width: 52ch;
  }
  .preview-bar {
    /* Contain the fixed-position ScopeBar for story review */
    position: relative;
    height: 60px;
    border: 1px dashed var(--color-border);
    border-radius: var(--radius-md);
    overflow: hidden;
  }
  .preview-bar :global(.scope-bar) {
    position: absolute;
    left: 0;
    right: 0;
  }
  .demo-content {
    font-size: var(--font-size-sm);
    color: var(--color-fg);
  }
  .demo-input {
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg);
    font-size: var(--font-size-xs);
    padding: 2px var(--space-2);
    width: 200px;
  }
  .window-label {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    white-space: nowrap;
  }
  .resolution-label {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .label-text {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--color-fg-subtle);
  }
  select {
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg);
    font-size: var(--font-size-xs);
    padding: 2px var(--space-3);
  }
  .notes {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  ul {
    margin: 0;
    padding-left: var(--space-4);
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    line-height: var(--line-height-base);
  }
</style>
