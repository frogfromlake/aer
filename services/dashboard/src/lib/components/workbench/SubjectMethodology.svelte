<script lang="ts">
  // SubjectMethodology — Phase 148g.
  //
  // The methodology for ONE bound subject of a cell: a metric or a categorical
  // field, with the role(s) it plays in the view (X axis, Size, Group by,
  // Facet, …). MeasureDetail enumerates the cell's subjects (cellSubjects) and
  // renders one of these per subject, so a scatter (x/y/size/colour), a
  // correlation matrix (a metric set), a sankey (a field chain) and a faceted
  // distribution all surface the full methodology of every dimension they bind
  // — not just the single-metric views.
  //
  // Each instance owns its own queries (a fixed set, enabled-gated by kind), so
  // a dynamic number of subjects is rendered safely via an {#each} in the parent
  // without breaking the query-per-render rule.
  import { createQuery } from '@tanstack/svelte-query';
  import { m } from '$lib/paraglide/messages.js';
  import {
    contentQuery,
    provenanceQuery,
    type ContentResponseDto,
    type FetchContext,
    type MetricProvenanceDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import Badge from '$lib/components/base/Badge.svelte';
  import ProgressiveSemantics from '$lib/components/ProgressiveSemantics.svelte';
  import { pickBadgeTier } from '$lib/components/chrome/methodology-tray-internals';
  import { cellContentId, hasCellMethodologyContent } from '$lib/presentations';
  import type { SubjectRole } from '$lib/presentations';
  import type { Presentation } from '$lib/state/url-internals';
  import {
    metricLabel,
    fieldLabel,
    fieldDescription,
    isRegisteredMetric
  } from '$lib/state/labels.svelte';
  import { locale } from '$lib/state/locale.svelte';

  interface Props {
    /** Machine name of the bound metric or field. */
    name: string;
    /** Role(s) this subject plays in the cell (X axis, Size, Group by, …). */
    roles: SubjectRole[];
    viewMode: Presentation;
    /** Human label of the active view (for the per-(view×metric) pairing fetch). */
    viewLabel: string;
  }
  let { name, roles, viewMode, viewLabel }: Props = $props();

  const ctx: FetchContext = { baseUrl: '/api/v1' };

  // Kind resolution — a real `/metrics/available` metric, else one of the fixed
  // 22 metadata fields, else unknown (render nothing rather than a broken fetch).
  const isMetric = $derived(isRegisteredMetric(name));
  const fieldDesc = $derived(fieldDescription(name));
  const isField = $derived(!isMetric && fieldDesc !== null);

  const subjectLabel = $derived(isMetric ? metricLabel(name) : fieldLabel(name));

  // Role chips ('primary' = the lone metric of a single-metric view → no chip).
  const ROLE_LABELS: Record<SubjectRole, (() => string) | null> = {
    primary: null,
    x: m.rg_role_x,
    y: m.rg_role_y,
    size: m.rg_role_size,
    color: m.rg_role_color,
    set: m.rg_role_set,
    groupBy: m.rg_role_group_by,
    aggregated: m.rg_role_aggregated,
    leading: m.rg_role_leading,
    lagging: m.rg_role_lagging,
    nodeSize: m.rg_role_node_size,
    nodeColor: m.rg_role_node_color,
    field: m.rg_role_field,
    chain: m.rg_role_chain,
    facet: m.rg_role_facet
  };
  const roleChips = $derived(
    roles.map((r) => ROLE_LABELS[r]).filter((f): f is () => string => !!f)
  );

  // ── Metric subject queries ──────────────────────────────────────────────────
  const provenanceQ = createQuery<
    QueryOutcome<MetricProvenanceDto>,
    Error,
    QueryOutcome<MetricProvenanceDto>
  >(() => {
    const o = provenanceQuery(ctx, name);
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: isMetric && name.length > 0
    };
  });

  const metricContentQ = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const o = contentQuery(ctx, 'metric', name, locale());
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: isMetric && name.length > 0
    };
  });

  // Per-(view×metric) pairing prose — only the single-metric views ship it.
  const hasPairing = $derived(isMetric && hasCellMethodologyContent(viewMode));
  const pairingId = $derived(cellContentId(viewMode, name));
  const pairingQ = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const o = contentQuery(ctx, 'view_mode', pairingId, locale());
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: hasPairing && name.length > 0
    };
  });

  // ── Field subject query (Phase 148g — Option A `field` content type) ─────────
  const fieldContentQ = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const o = contentQuery(ctx, 'field', name, locale());
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: isField && name.length > 0
    };
  });

  const provenance = $derived<MetricProvenanceDto | null>(
    provenanceQ.data?.kind === 'success' ? provenanceQ.data.data : null
  );
  const metricContent = $derived<ContentResponseDto | null>(
    metricContentQ.data?.kind === 'success' ? metricContentQ.data.data : null
  );
  const pairingContent = $derived<ContentResponseDto | null>(
    pairingQ.data?.kind === 'success' ? pairingQ.data.data : null
  );
  const fieldContent = $derived<ContentResponseDto | null>(
    fieldContentQ.data?.kind === 'success' ? fieldContentQ.data.data : null
  );

  const badgeTier = $derived(pickBadgeTier(provenance));
  const hasLimitations = $derived((provenance?.knownLimitations.length ?? 0) > 0);
</script>

{#if isMetric || isField}
  <div class="subject-block" data-kind={isMetric ? 'metric' : 'field'}>
    <div class="subject-head">
      {#each roleChips as chip (chip)}
        <span class="role-chip">{chip()}</span>
      {/each}
      <code class="subject-name">{subjectLabel}</code>
      {#if isMetric}
        <Badge tier={badgeTier} />
      {/if}
      {#if hasLimitations}
        <span class="limitations-pill" title={m.workbench_meth_known_limitations_pill_title()}>
          {m.workbench_meth_known_limitations_pill()}
        </span>
      {/if}
    </div>

    {#if isMetric}
      {#if provenanceQ.isPending || metricContentQ.isPending}
        <p class="muted" aria-busy="true">{m.workbench_meth_loading()}</p>
      {:else}
        {#if pairingContent}
          <details class="meth-block" data-section="cell-method">
            <summary class="meth-block-summary"
              >{m.workbench_meth_cell_method_heading({
                view: viewLabel,
                metric: subjectLabel
              })}</summary
            >
            <p class="cell-method-text">{pairingContent.registers.methodological.long}</p>
          </details>
        {/if}

        {#if metricContent}
          <details class="meth-block" data-section="dual-register">
            <summary class="meth-block-summary">{m.workbench_meth_what_metric_measures()}</summary>
            <ProgressiveSemantics registers={metricContent.registers} emphasis="methodological" />
          </details>
        {/if}

        {#if provenance}
          <details class="meth-block" data-section="provenance">
            <summary class="meth-block-summary">{m.workbench_meth_provenance()}</summary>
            <dl class="provenance-dl">
              <dt>{m.workbench_meth_provenance_tier()}</dt>
              <dd><Badge tier={badgeTier} /></dd>
              <dt>{m.workbench_meth_provenance_validation()}</dt>
              <dd class="status status-{provenance.validationStatus}">
                {provenance.validationStatus}
              </dd>
              <dt>{m.workbench_meth_provenance_algorithm()}</dt>
              <dd>{provenance.algorithmDescription}</dd>
              <dt>{m.workbench_meth_provenance_extractor()}</dt>
              <dd><code>{provenance.extractorVersionHash}</code></dd>
            </dl>
            {#if provenance.culturalContextNotes}
              <p class="cultural-notes">{provenance.culturalContextNotes}</p>
            {/if}
          </details>
        {/if}

        {#if hasLimitations && provenance}
          <details class="meth-block" data-section="limitations">
            <summary class="meth-block-summary">{m.workbench_meth_known_limitations()}</summary>
            <ul class="limitations-list">
              {#each provenance.knownLimitations as lim (lim)}
                <li>{lim}</li>
              {/each}
            </ul>
          </details>
        {/if}

        <div class="meth-links">
          <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- internal Reflection route -->
          <a class="meth-link" href="/reflection/metric/{name}">
            {m.workbench_meth_link_provenance_page()}
          </a>
        </div>
      {/if}
    {:else if isField}
      <details class="meth-block" data-section="field" open>
        <summary class="meth-block-summary">{m.workbench_meth_what_field_means()}</summary>
        {#if fieldContent}
          <ProgressiveSemantics registers={fieldContent.registers} emphasis="methodological" />
        {:else}
          <!-- Option A field content not yet present → the curated one-liner. -->
          <p class="cell-method-text">{fieldDesc ?? m.workbench_meth_field_no_desc()}</p>
        {/if}
      </details>
    {/if}
  </div>
{/if}

<style>
  .subject-block {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding-top: var(--space-3);
    border-top: 1px dashed var(--color-border);
  }
  .subject-block:first-child {
    border-top: none;
    padding-top: 0;
  }

  .subject-head {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-2);
  }
  .role-chip {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    padding: 1px var(--space-2);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-pill);
    color: var(--color-fg-muted);
    font-family: var(--font-mono);
  }
  .subject-name {
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    color: var(--color-fg);
  }

  .limitations-pill {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    padding: 1px var(--space-2);
    border: 1px solid var(--color-status-expired);
    color: var(--color-status-expired);
    border-radius: var(--radius-pill);
    font-family: var(--font-mono);
  }

  .meth-block {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .meth-block-summary {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: 0;
    list-style: none;
    cursor: pointer;
    font-size: var(--font-size-xs);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg-subtle);
  }
  .meth-block-summary::-webkit-details-marker {
    display: none;
  }
  .meth-block-summary::before {
    content: '›';
    display: inline-block;
    color: var(--color-fg-subtle);
    transition: transform var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .meth-block[open] > .meth-block-summary::before {
    transform: rotate(90deg);
  }
  .meth-block-summary:hover {
    color: var(--color-fg);
  }

  .limitations-list {
    margin: 0;
    padding-left: var(--space-5);
    font-size: var(--font-size-sm);
    color: var(--color-fg);
    line-height: var(--line-height-loose);
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .provenance-dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: var(--space-2) var(--space-4);
    margin: 0;
    font-size: var(--font-size-sm);
  }
  .provenance-dl dt {
    color: var(--color-fg-muted);
    font-weight: var(--font-weight-medium);
  }
  .provenance-dl dd {
    margin: 0;
    color: var(--color-fg);
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .provenance-dl dd code {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
  }

  .status {
    display: inline-block;
    padding: 1px 6px;
    border-radius: 10px;
    font-size: 11px;
    text-transform: uppercase;
  }
  .status-unvalidated {
    color: #caa04a;
    background: rgba(202, 160, 74, 0.12);
  }
  .status-validated {
    color: #4ca84c;
    background: rgba(76, 168, 76, 0.12);
  }
  .status-expired {
    color: #c06060;
    background: rgba(192, 96, 96, 0.12);
  }

  .cultural-notes {
    margin: var(--space-2) 0 0;
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
  }
  .cell-method-text {
    font-size: var(--font-size-sm);
    color: var(--color-fg);
    line-height: var(--line-height-loose);
    margin: 0;
  }

  .meth-links {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding-top: var(--space-2);
  }
  .meth-link {
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-accent);
    text-decoration: none;
    border-bottom: 1px dotted var(--color-accent-muted);
    align-self: flex-start;
  }
  .meth-link:hover,
  .meth-link:focus-visible {
    color: var(--color-fg);
    border-bottom-style: solid;
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  @media (prefers-reduced-motion: reduce) {
    .meth-block-summary::before {
      transition: none;
    }
  }
</style>
