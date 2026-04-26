<script lang="ts">
  // Silver-ineligibility panel (Phase 111).
  // Renders when the Silver-layer toggle is active but the active source
  // has not passed the WP-006 §5.2 eligibility review. Shown instead of
  // the view-mode matrix to make the governance boundary explicit rather
  // than silently omitting data.
  //
  // The panel is intentionally static (no BFF query) because the
  // eligibility state is already carried in the ProbeDossier payload.
  // The WP-006 §5.2 reference gives the user a methodological anchor
  // without requiring a round-trip.
  import type { ProbeDossierSourceDto } from '$lib/api/queries';

  interface Props {
    source: ProbeDossierSourceDto;
  }

  let { source }: Props = $props();
</script>

<section
  class="ineligible-panel"
  role="status"
  aria-live="polite"
  aria-labelledby="ineligible-title"
>
  <header class="panel-header">
    <span class="badge" aria-label="Silver access not granted">Not Silver-eligible</span>
    <code class="source-name">{source.name}</code>
  </header>

  <p class="body">
    Access to Silver-layer document data for <strong>{source.name}</strong> has not been granted. Silver-layer
    self-service access requires a completed WP-006 §5.2 review. Until that review is on record, this
    source's Silver data is withheld to uphold the governance boundary between aggregated observation
    (Gold) and individual document access (Silver).
  </p>

  <p class="body">
    To request eligibility review, open the WP-006 §5.2 assessment process. The outcome — reviewer,
    date, rationale, and reference — will be recorded in the source registry and reflected here.
  </p>

  <dl class="meta">
    {#if source.silverReviewDate}
      <div>
        <dt>Last review</dt>
        <dd>{source.silverReviewDate}</dd>
      </div>
    {/if}
    <div>
      <dt>Status</dt>
      <dd>Ineligible (WP-006 §5.2)</dd>
    </div>
    <div>
      <dt>Source type</dt>
      <dd>{source.type}</dd>
    </div>
  </dl>
</section>

<style>
  .ineligible-panel {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    padding: var(--space-5);
    border: 1px solid var(--color-border);
    border-left: 3px solid #7ec4a0;
    border-radius: var(--radius-md);
    background: var(--color-surface);
    max-width: 48rem;
  }

  .panel-header {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
  }

  .badge {
    display: inline-block;
    padding: 2px var(--space-2);
    border-radius: var(--radius-sm);
    background: rgba(126, 196, 160, 0.15);
    border: 1px solid #7ec4a0;
    color: #7ec4a0;
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    font-weight: var(--font-weight-semibold);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }

  .source-name {
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
  }

  .body {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
    margin: 0;
  }

  .body strong {
    color: var(--color-fg);
    font-weight: var(--font-weight-medium);
  }

  .meta {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-4);
    margin: 0;
    padding-top: var(--space-3);
    border-top: 1px solid var(--color-border);
  }

  .meta div {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .meta dt {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-subtle);
    font-family: var(--font-mono);
  }

  .meta dd {
    margin: 0;
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-fg-muted);
  }
</style>
