<script lang="ts">
  // Phase 115 — Probe Dossier "valid comparisons" panel.
  //
  // Reads `GET /api/v1/probes/{probeId}/equivalence` and renders the
  // per-metric Level-1 / Level-2 / Level-3 availability as a small
  // matrix on the Dossier — making the methodological boundary of the
  // probe legible up front, before the user has to encounter a refusal
  // in a function lane (Brief §7.4 + WP-004 §6.3).
  import { createQuery } from '@tanstack/svelte-query';
  import {
    probeEquivalenceQuery,
    type FetchContext,
    type ProbeEquivalenceDto,
    type QueryOutcome
  } from '$lib/api/queries';

  interface Props {
    probeId: string;
    ctx?: FetchContext;
  }

  let { probeId, ctx = { baseUrl: '/api/v1' } }: Props = $props();

  const query = createQuery<
    QueryOutcome<ProbeEquivalenceDto>,
    Error,
    QueryOutcome<ProbeEquivalenceDto>
  >(() => {
    const o = probeEquivalenceQuery(ctx, probeId);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });
</script>

<section class="valid-comparisons" aria-label="Valid comparisons for this probe">
  <header>
    <h3>Valid comparisons</h3>
    <p class="lead">
      Each row is a metric available in this probe's scope. The dots show which cross-context
      comparison levels (WP-004 §5.3) the equivalence registry has validated.
    </p>
  </header>

  {#if query.isPending}
    <p class="muted" aria-busy="true">Loading equivalence registry…</p>
  {:else if query.isError || query.data?.kind !== 'success'}
    <p class="muted">Equivalence summary unavailable.</p>
  {:else if query.data.data.metrics.length === 0}
    <p class="muted">No metrics with data in the active window.</p>
  {:else}
    <table>
      <thead>
        <tr>
          <th scope="col">Metric</th>
          <th scope="col" title="Temporal patterns — always intra-culturally valid">L1</th>
          <th
            scope="col"
            title="Z-score / percentile deviation — requires deviation-level equivalence"
          >
            L2
          </th>
          <th scope="col" title="Absolute values — requires absolute-level equivalence">L3</th>
          <th scope="col" class="notes-col">Notes</th>
        </tr>
      </thead>
      <tbody>
        {#each query.data.data.metrics as m (m.metricName)}
          <tr>
            <th scope="row" class="metric-name">{m.metricName}</th>
            <td class:dot-on={m.level1Available}>
              <span class="dot" aria-label={m.level1Available ? 'available' : 'unavailable'}></span>
            </td>
            <td class:dot-on={m.level2Available}>
              <span class="dot" aria-label={m.level2Available ? 'available' : 'unavailable'}></span>
            </td>
            <td class:dot-on={m.level3Available}>
              <span class="dot" aria-label={m.level3Available ? 'available' : 'unavailable'}></span>
            </td>
            <td class="notes-cell">
              {#if m.equivalenceStatus?.notes}
                <span class="notes" title={m.equivalenceStatus.notes}
                  >{m.equivalenceStatus.notes}</span
                >
              {:else}
                <span class="muted">—</span>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</section>

<style>
  .valid-comparisons {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    padding: var(--space-4);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: var(--color-surface);
  }
  h3 {
    margin: 0;
    font-size: var(--font-size-base);
  }
  .lead {
    margin: 0;
    color: var(--color-fg-muted);
    font-size: var(--font-size-sm);
    line-height: 1.55;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--font-size-sm);
  }
  th,
  td {
    padding: var(--space-1) var(--space-2);
    text-align: left;
    border-bottom: 1px solid var(--color-border);
  }
  thead th {
    color: var(--color-fg-muted);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    font-size: var(--font-size-xs);
  }
  .metric-name {
    font-family: var(--font-family-mono);
    font-weight: normal;
  }
  .notes-col {
    width: 40%;
  }
  .dot {
    display: inline-block;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: transparent;
    border: 1px solid var(--color-fg-muted);
  }
  .dot-on .dot {
    background: var(--color-accent);
    border-color: var(--color-accent);
  }
  .notes {
    color: var(--color-fg);
  }
  .muted {
    color: var(--color-fg-muted);
  }
</style>
