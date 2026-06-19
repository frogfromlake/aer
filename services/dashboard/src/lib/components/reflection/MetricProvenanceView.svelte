<script lang="ts">
  // MetricProvenanceView — the reusable provenance body of a metric (Phase 127).
  // Extracted from reflection/metric/[name]/+page.svelte so the singular page and
  // the /reflection/metrics aggregate render identical content. Owns the
  // dual-register prose, the provenance detail list, known limitations, and WP
  // cross-references; the page shell owns the title/badge header and back chrome.
  import Badge from '$lib/components/base/Badge.svelte';
  import ProgressiveSemantics from '$lib/components/ProgressiveSemantics.svelte';
  import { pickBadgeTier } from '$lib/components/chrome/methodology-tray-internals';
  import type { MetricProvenanceDto, ContentResponseDto } from '$lib/api/queries';
  import { m } from '$lib/paraglide/messages.js';

  interface Props {
    provenance: MetricProvenanceDto | null;
    contentRecord: ContentResponseDto | null;
  }
  let { provenance, contentRecord }: Props = $props();

  const badgeTier = $derived(pickBadgeTier(provenance));
</script>

<!-- Dual-register content -->
{#if contentRecord}
  <section class="prov-section" aria-labelledby="registers-heading">
    <h3 id="registers-heading" class="section-title">{m.reflection_metric_measures_heading()}</h3>
    <ProgressiveSemantics registers={contentRecord.registers} emphasis="methodological" />
  </section>
{/if}

<!-- Provenance details -->
{#if provenance}
  <section class="prov-section" aria-labelledby="prov-heading">
    <h3 id="prov-heading" class="section-title">{m.reflection_metric_details_heading()}</h3>
    <dl class="prov-dl">
      <dt>{m.reflection_metric_detail_tier()}</dt>
      <dd>
        <Badge tier={badgeTier} />
        {m.reflection_metric_detail_tier_value({ tier: provenance.tierClassification })}
      </dd>

      <dt>{m.reflection_metric_detail_validation()}</dt>
      <dd class="status-{provenance.validationStatus}">{provenance.validationStatus}</dd>

      <dt>{m.reflection_metric_detail_algorithm()}</dt>
      <dd>{provenance.algorithmDescription}</dd>

      <dt>{m.reflection_metric_detail_extractor_version()}</dt>
      <dd><code class="mono">{provenance.extractorVersionHash}</code></dd>

      {#if provenance.culturalContextNotes}
        <dt>{m.reflection_metric_detail_cultural_context()}</dt>
        <dd>{provenance.culturalContextNotes}</dd>
      {/if}
    </dl>
  </section>

  {#if provenance.knownLimitations.length > 0}
    <section class="prov-section limitations-section" aria-labelledby="limits-heading">
      <h3 id="limits-heading" class="section-title">{m.reflection_metric_limitations_heading()}</h3>
      <ul class="limits-list">
        {#each provenance.knownLimitations as lim (lim)}
          <li>{lim}</li>
        {/each}
      </ul>
    </section>
  {/if}
{/if}

<!-- WP cross-references from content catalog -->
{#if contentRecord?.workingPaperAnchors && contentRecord.workingPaperAnchors.length > 0}
  <section class="prov-section" aria-labelledby="wp-refs-heading">
    <h3 id="wp-refs-heading" class="section-title">{m.reflection_metric_wp_refs_heading()}</h3>
    <ul class="wp-ref-list" role="list">
      {#each contentRecord.workingPaperAnchors as anchor (anchor)}
        {@const parts = anchor.match(/^(WP-\d+)\s*§?\s*(.*)$/i)}
        {#if parts}
          {@const wpId = (parts[1] ?? '').toLowerCase()}
          {@const section = (parts[2] ?? '').trim()}
          <li>
            <!-- eslint-disable svelte/no-navigation-without-resolve -->
            <a
              href="/reflection/wp/{wpId}{section ? `?section=${encodeURIComponent(section)}` : ''}"
              class="wp-ref-link">{anchor}</a
            >
            <!-- eslint-enable svelte/no-navigation-without-resolve -->
          </li>
        {:else}
          <li class="wp-ref-raw">{anchor}</li>
        {/if}
      {/each}
    </ul>
  </section>
{/if}

<style>
  .prov-section {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .section-title {
    font-size: var(--font-size-lg);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    margin: 0;
  }

  /* Provenance DL */
  .prov-dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: var(--space-2) var(--space-5);
    font-size: var(--font-size-sm);
    padding: var(--space-4);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }

  .prov-dl dt {
    color: var(--color-fg-muted);
    font-weight: var(--font-weight-medium);
  }

  .prov-dl dd {
    margin: 0;
    color: var(--color-fg);
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .mono {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
  }

  .status-unvalidated {
    color: var(--color-status-unvalidated);
  }

  .status-validated {
    color: var(--color-status-validated);
  }

  .status-expired {
    color: var(--color-status-expired);
  }

  /* Known limitations */
  .limitations-section {
    padding: var(--space-4);
    background: rgba(192, 96, 96, 0.06);
    border: 1px solid rgba(192, 96, 96, 0.2);
    border-radius: var(--radius-md);
  }

  .limits-list {
    margin: 0;
    padding-left: var(--space-5);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .limits-list li {
    font-size: var(--font-size-sm);
    line-height: var(--line-height-loose);
    color: var(--color-fg);
  }

  /* WP refs */
  .wp-ref-list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .wp-ref-link {
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    color: var(--color-accent);
    text-decoration: none;
  }

  .wp-ref-link:hover {
    text-decoration: underline;
  }

  .wp-ref-raw {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
  }
</style>
