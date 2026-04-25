<script lang="ts">
  import SideRail from '$lib/components/chrome/SideRail.svelte';
</script>

<div class="story">
  <h2>SideRail</h2>
  <p class="desc">
    Persistent left navigation rail (Design Brief §3.2). Three surface anchors, return-to-Atmosphere
    planet glyph, compact scope indicator, and pillar-mode toggle. Fixed-position in production;
    shown inline here for visual review.
  </p>

  <div class="preview">
    <div class="rail-frame">
      <SideRail />
    </div>
    <div class="notes">
      <h3>States</h3>
      <ul>
        <li>
          <strong>Surface anchors</strong> — Atmosphere (●), Function Lanes (≡), Reflection (¶)
        </li>
        <li>
          <strong>Active surface</strong> — highlighted when URL matches /stories/chrome/side-rail
        </li>
        <li>
          <strong>Scope indicator</strong> — shows active probe ID truncated to 7 chars; "—" when no probe
          in URL
        </li>
        <li>
          <strong>Pillar toggle</strong> — A / E / R; reads and writes ?viewingMode= URL param
        </li>
        <li><strong>Planet glyph</strong> — ◉ in accent color, links to /</li>
      </ul>
      <h3>A11y</h3>
      <ul>
        <li>nav[aria-label="Primary navigation"]</li>
        <li>Surface links: aria-current="page" on active; aria-label per surface</li>
        <li>Pillar buttons: role="radio" + aria-checked + sr-only label text</li>
        <li>Reduced-motion: all transitions suppressed via prefers-reduced-motion</li>
      </ul>
    </div>
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
    margin: 0 0 var(--space-2);
  }
  .desc {
    color: var(--color-fg-muted);
    font-size: var(--font-size-sm);
    line-height: var(--line-height-loose);
    margin: 0;
    max-width: 52ch;
  }
  .preview {
    display: flex;
    gap: var(--space-6);
    align-items: flex-start;
  }
  .rail-frame {
    /* In production SideRail is position:fixed. Here we contain it with
       position:relative on the frame so it renders inline for the story. */
    position: relative;
    width: 52px;
    height: 400px;
    border: 1px dashed var(--color-border);
    border-radius: var(--radius-md);
    overflow: hidden;
    flex-shrink: 0;
  }

  /* Override the rail's fixed positioning for the story preview */
  .rail-frame :global(.rail) {
    position: absolute;
  }
  .notes {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
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
