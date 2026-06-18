<script lang="ts">
  // ProbeCard — Phase 122k K2.
  //
  // Canonical Probe view inside the Dossier catalogue. Absorbs the
  // Phase-122i ProbeDossier component (now deleted) and restructures:
  //
  //   * Header: probe identity + `[→ Analyse in Workbench]` CTA +
  //     `[Metadata coverage]` modal trigger + expand toggle
  //   * Emic frame paragraph (semantic register from BFF /content)
  //   * Structural meta (sources, publication rate, function coverage)
  //   * Capability matrix (what AĒR CAN compute — no result claim)
  //   * Discourse-Function Cards (the ProbeDfCards child) — each DF a
  //     collapsable container holding the Source-Cards classified under it.
  //
  // Phase 141 — the DF-Cards region + its grouping/coverage logic moved to
  // `ProbeDfCards.svelte` + `probe-card-internals.ts`; this file is the card
  // shell (queries, header, structural meta, capabilities).
  import { createQuery } from '@tanstack/svelte-query';
  import {
    contentQuery,
    probeDossierQuery,
    type ContentResponseDto,
    type FetchContext,
    type ProbeDossierDto,
    type ProbeDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import MetadataCoverageModal from './MetadataCoverageModal.svelte';
  import ProbeDfCards from './ProbeDfCards.svelte';
  import {
    coveragePercent as computeCoveragePercent,
    publicationRatePerDay as computePublicationRate
  } from './probe-card-internals';
  import { locale } from '$lib/state/locale.svelte';
  import { m } from '$lib/paraglide/messages.js';
  import { formatNumber } from '$lib/localization/format';

  interface Props {
    probe: ProbeDto;
    ctx: FetchContext;
    /** Phase 131a — `undefined` ⇒ "no window filter, show whole dataset"
     *  (BFF treats absent bounds as no filter; `in_window == total` and
     *  the per-day rate falls back to the long-run pub-date span). The
     *  Phase-123a date-range picker passes concrete ISO strings here. */
    windowStart: string | undefined;
    windowEnd: string | undefined;
    /** Driven by the `/dossier?expand=<id>` deep-link reader. */
    startCollapsed?: boolean;
  }

  let { probe, ctx, windowStart, windowEnd, startCollapsed = false }: Props = $props();

  // Phase 122k — expansion tracks the `startCollapsed` prop. When the
  // Probe-Filter Modal updates `url.selectedProbes` and the parent page
  // re-derives which probes should auto-expand, the writable-derived
  // pattern re-evaluates the base value while still letting the user's
  // header click stamp a local override.
  let expanded = $derived(!startCollapsed);

  // Metadata-Coverage Modal open state.
  let mdcOpen = $state(false);

  const dossierQ = createQuery<QueryOutcome<ProbeDossierDto>, Error, QueryOutcome<ProbeDossierDto>>(
    () => {
      const o = probeDossierQuery(ctx, probe.probeId, { windowStart, windowEnd });
      return {
        queryKey: [...o.queryKey],
        queryFn: o.queryFn,
        staleTime: o.staleTime,
        enabled: probe.probeId !== ''
      };
    }
  );

  const probeContentQ = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const o = contentQuery(ctx, 'probe', probe.probeId, locale());
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  const dossier = $derived<ProbeDossierDto | null>(
    dossierQ.data?.kind === 'success' ? dossierQ.data.data : null
  );

  const publicationRatePerDay = $derived(dossier ? computePublicationRate(dossier.sources) : null);

  const coveragePercent = $derived(dossier ? computeCoveragePercent(dossier.functionCoverage) : 0);

  function toggleExpanded() {
    expanded = !expanded;
  }

  function openMetadataModal() {
    mdcOpen = true;
  }
</script>

<article
  class="probe-card"
  id="probe-{probe.probeId}"
  class:expanded
  aria-labelledby="pc-title-{probe.probeId}"
>
  <header class="probe-header">
    <button
      type="button"
      class="header-toggle"
      onclick={toggleExpanded}
      aria-expanded={expanded}
      aria-controls="pc-body-{probe.probeId}"
    >
      <span class="chevron" class:expanded aria-hidden="true">›</span>
      <h2 id="pc-title-{probe.probeId}" class="probe-title">{probe.displayName}</h2>
      <code class="probe-id" aria-label={m.dossier_probe_machine_id_aria_label()}
        >{probe.probeId}</code
      >
      <span
        class="lang-badge"
        aria-label={m.dossier_probe_language_aria_label({ language: probe.language })}
      >
        {probe.language}
      </span>
      {#if dossier}
        <span class="summary" aria-hidden={expanded ? 'true' : 'false'}>
          {(dossier.sources.length === 1
            ? m.dossier_probe_summary_one
            : m.dossier_probe_summary_other)({
            sources: dossier.sources.length,
            covered: dossier.functionCoverage.covered,
            total: dossier.functionCoverage.total
          })}
        </span>
      {/if}
    </button>

    <div class="header-actions">
      <!-- Phase 123c (TESTING.md §3 Issue) — direct nav to the per-probe
           methodology dossier, previously reachable only by hand-typed URL. -->
      <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- internal reflection route -->
      <a class="methodology-link" href="/reflection/probe/{probe.probeId}"
        >{m.dossier_probe_methodology_link()}</a
      >
      <button type="button" class="metadata-btn" onclick={openMetadataModal}>
        {m.dossier_probe_metadata_button()}
      </button>
    </div>
  </header>

  {#if expanded}
    <div class="probe-body" id="pc-body-{probe.probeId}">
      {#if dossierQ.isPending}
        <p class="muted" aria-busy="true">{m.common_loading()}</p>
      {:else if !dossier}
        <p class="error">{m.dossier_probe_load_failed({ probeId: probe.probeId })}</p>
      {:else}
        <!-- Emic frame -->
        {#if probeContentQ.data?.kind === 'success'}
          <p class="emic-frame">{probeContentQ.data.data.registers.semantic.long}</p>
        {/if}

        <!-- Structural meta -->
        <dl class="meta">
          <div>
            <dt>{m.dossier_probe_meta_language()}</dt>
            <dd>{dossier.language.toUpperCase()}</dd>
          </div>
          <div>
            <dt>{m.dossier_probe_meta_sources()}</dt>
            <dd>{formatNumber(dossier.sources.length)}</dd>
          </div>
          <div>
            <dt>{m.dossier_probe_meta_publication_rate()}</dt>
            <dd>
              {publicationRatePerDay !== null
                ? m.dossier_probe_meta_publication_rate_value({
                    rate: formatNumber(publicationRatePerDay, {
                      minimumFractionDigits: 1,
                      maximumFractionDigits: 1
                    })
                  })
                : '—'}
            </dd>
          </div>
          <div>
            <dt>{m.dossier_probe_meta_function_coverage()}</dt>
            <dd>
              {m.dossier_probe_meta_function_coverage_value({
                covered: dossier.functionCoverage.covered,
                total: dossier.functionCoverage.total,
                percent: coveragePercent
              })}
            </dd>
          </div>
        </dl>

        <!-- Phase 123a — capability matrix. What AĒR CAN compute for this
             probe (no result claim). Universal/structural — never a
             cross-probe ranking. -->
        {#if dossier.capabilities}
          {@const caps = dossier.capabilities}
          <section class="capabilities" aria-labelledby="cap-heading-{probe.probeId}">
            <h3 id="cap-heading-{probe.probeId}" class="section-title">
              {m.dossier_probe_capabilities_title()}
            </h3>
            <dl class="meta">
              <div>
                <dt>{m.dossier_probe_cap_sentiment_backbone()}</dt>
                <dd>{caps.sentimentBackbone ?? '—'}</dd>
              </div>
              <div>
                <dt>{m.dossier_probe_cap_sentiment_enrichments()}</dt>
                <dd>
                  {caps.sentimentEnrichments.length > 0
                    ? caps.sentimentEnrichments.join(' · ')
                    : '—'}
                </dd>
              </div>
              <div>
                <dt>{m.dossier_probe_cap_silent_edit()}</dt>
                <dd>
                  {caps.silentEditObservability
                    ? m.dossier_probe_cap_silent_edit_active()
                    : m.dossier_probe_cap_silent_edit_inactive()}
                </dd>
              </div>
              <div>
                <dt>{m.dossier_probe_cap_discourse_function()}</dt>
                <dd>{caps.discourseFunctionClassifier}</dd>
              </div>
            </dl>
          </section>
        {/if}

        <ProbeDfCards
          probeId={probe.probeId}
          sources={dossier.sources}
          {ctx}
          {windowStart}
          {windowEnd}
        />
      {/if}
    </div>
  {/if}
</article>

{#if mdcOpen && dossier}
  <MetadataCoverageModal probeId={probe.probeId} {ctx} onClose={() => (mdcOpen = false)} />
{/if}

<style>
  .probe-card {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    padding: var(--space-4);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }

  .probe-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    flex-wrap: wrap;
  }

  .header-toggle {
    appearance: none;
    background: transparent;
    border: none;
    color: inherit;
    cursor: pointer;
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
    flex: 1 1 auto;
    text-align: left;
    padding: 0;
  }

  .chevron {
    color: var(--color-fg-subtle);
    transition: transform var(--motion-duration-fast) var(--motion-ease-standard);
    display: inline-block;
  }
  .chevron.expanded {
    transform: rotate(90deg);
  }

  .probe-title {
    margin: 0;
    font-size: var(--font-size-base);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
  }

  /* Phase 123 — the machine probeId as a muted, auditable subtitle next to
     the human-friendly display name. */
  .probe-id {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
  }

  .lang-badge {
    font-size: var(--font-size-xs);
    padding: 1px var(--space-2);
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-pill);
    color: var(--color-fg-muted);
    font-family: var(--font-mono);
    text-transform: uppercase;
  }

  .summary {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    font-family: var(--font-mono);
  }

  .header-actions {
    display: flex;
    gap: var(--space-2);
    flex-shrink: 0;
  }

  .metadata-btn,
  .methodology-link {
    appearance: none;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 4px var(--space-3);
    font-size: var(--font-size-xs);
    cursor: pointer;
    background: transparent;
    color: var(--color-fg-muted);
    text-decoration: none;
    white-space: nowrap;
  }
  .metadata-btn:hover,
  .metadata-btn:focus-visible,
  .methodology-link:hover,
  .methodology-link:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
  }

  .probe-body {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    padding-top: var(--space-2);
    border-top: 1px solid var(--color-border);
  }

  .emic-frame {
    margin: 0;
    color: var(--color-fg);
    font-size: var(--font-size-sm);
    line-height: 1.6;
    max-width: 60rem;
  }

  .meta {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
    gap: var(--space-3);
    margin: 0;
  }
  .meta > div {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .meta dt {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
  }
  .meta dd {
    margin: 0;
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    color: var(--color-fg);
  }

  .section-title {
    margin: 0 0 var(--space-1) 0;
    font-size: var(--font-size-md);
    color: var(--color-fg);
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  .error {
    font-size: var(--font-size-sm);
    color: var(--color-status-expired);
    margin: 0;
  }
</style>
