<script lang="ts">
  // Live text fallback for browsers without WebGL2 (ROADMAP Phase 99b).
  //
  // The 99a version of this component was a static notice. 99b extends it
  // with the same live data the 3D surface shows: active probes, their
  // language + emission-point labels, and current publication rate.
  //
  // Reach is deliberately absent. The text surface mirrors the visual
  // one: AĒR does not claim where a probe's content is read.
  import type { ProbeActivity } from '@aer/engine-3d';
  import type { ProbeDto } from '$lib/api/queries';

  interface Props {
    /** Active probes from `/api/v1/probes`. Empty while loading. */
    probes: ProbeDto[];
    /** Per-probe activity (documents/hour) for the current time window. */
    activity: ProbeActivity[];
    /** True while the probes query is still in flight on first load. */
    loading?: boolean;
  }

  let { probes, activity, loading = false }: Props = $props();

  const activityByProbe = $derived(new Map(activity.map((a) => [a.probeId, a.documentsPerHour])));

  function formatRate(docsPerHour: number | undefined): string {
    if (docsPerHour === undefined || !Number.isFinite(docsPerHour)) return '—';
    if (docsPerHour === 0) return '0 docs/hour';
    if (docsPerHour < 0.1) return '< 0.1 docs/hour';
    return `${docsPerHour.toFixed(1)} docs/hour`;
  }
</script>

<section class="fallback" aria-labelledby="fallback-title">
  <header>
    <h1 id="fallback-title">ἀήρ</h1>
    <p class="lede">Atmospheric sensor for human discourse.</p>
    <p class="notice">
      Your browser does not support WebGL2, so the 3D atmosphere is unavailable. The live probe
      state below is the same data the visual surface would render.
    </p>
  </header>

  <section class="probes" aria-labelledby="probes-heading">
    <h2 id="probes-heading">Active probes</h2>

    {#if loading && probes.length === 0}
      <p class="muted" aria-busy="true">Loading active probes…</p>
    {:else if probes.length === 0}
      <p class="muted">No active probes.</p>
    {:else}
      <ul>
        {#each probes as probe (probe.probeId)}
          {@const rate = activityByProbe.get(probe.probeId)}
          <li>
            <div class="probe-head">
              <code class="probe-id">{probe.probeId}</code>
              <span class="lang" aria-label="Language">{probe.language}</span>
            </div>

            <dl>
              <dt>Emission points</dt>
              <dd>
                <ul class="points">
                  {#each probe.emissionPoints as ep (ep.label)}
                    <li>{ep.label}</li>
                  {/each}
                </ul>
              </dd>
              <dt>Publication rate</dt>
              <dd>{formatRate(rate)}</dd>
            </dl>
          </li>
        {/each}
      </ul>
    {/if}

    <p class="reach-note">
      Reach is not reported. A probe's emission points mark where its bound publishers emit — not
      where their content is consumed or influential. No reach claim is made by AĒR.
    </p>
  </section>
</section>

<style>
  .fallback {
    max-width: 44rem;
    margin: 0 auto;
    padding: var(--space-6);
    color: var(--color-fg);
  }
  header {
    text-align: center;
    margin-bottom: var(--space-7);
  }
  h1 {
    font-size: var(--font-size-4xl);
    margin: 0 0 var(--space-4);
  }
  .lede {
    color: var(--color-fg-muted);
    font-size: var(--font-size-md);
    margin: 0 0 var(--space-5);
  }
  .notice {
    color: var(--color-fg-subtle);
    font-size: var(--font-size-sm);
    line-height: var(--line-height-loose);
    max-width: 36rem;
    margin: 0 auto;
  }
  .probes {
    margin-top: var(--space-6);
  }
  h2 {
    font-size: var(--font-size-lg);
    margin: 0 0 var(--space-4);
  }
  .probes > ul {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  .probes > ul > li {
    padding: var(--space-4);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: var(--color-bg-raised, rgba(255, 255, 255, 0.02));
  }
  .probe-head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--space-3);
    margin-bottom: var(--space-3);
  }
  .probe-id {
    font-family: var(--font-family-mono);
    font-size: var(--font-size-sm);
    color: var(--color-fg);
  }
  .lang {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: var(--space-2) var(--space-4);
    margin: 0;
    font-size: var(--font-size-sm);
  }
  dt {
    color: var(--color-fg-muted);
  }
  dd {
    margin: 0;
    color: var(--color-fg);
  }
  .points {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1) var(--space-3);
  }
  .muted {
    color: var(--color-fg-muted);
    font-size: var(--font-size-sm);
  }
  .reach-note {
    margin-top: var(--space-5);
    padding-top: var(--space-4);
    border-top: 1px solid var(--color-border);
    color: var(--color-fg-muted);
    font-size: var(--font-size-xs);
    line-height: 1.55;
  }
</style>
