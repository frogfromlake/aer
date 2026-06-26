<script lang="ts">
  // Layperson-friendly "How AĒR works" data-flow diagram, shown inside the
  // About overlay. A six-step vertical flow (a numbered spine) from a public web
  // page to a questionable number — deliberately non-technical: no service names,
  // no protocols, just what happens to the data and why the trail stays
  // inspectable. The two storage steps carry the medallion layer tag (Bronze /
  // Silver · Gold) so the architecture's one load-bearing idea — keep the raw
  // evidence next to every result — is legible without a glossary.
  import { m } from '$lib/paraglide/messages.js';

  // Called accessors in a $derived array so the steps re-render on a locale flip.
  const steps = $derived([
    { title: m.about_flow_collect_title(), body: m.about_flow_collect_body(), layer: null },
    { title: m.about_flow_ingest_title(), body: m.about_flow_ingest_body(), layer: 'Bronze' },
    { title: m.about_flow_analyse_title(), body: m.about_flow_analyse_body(), layer: null },
    {
      title: m.about_flow_refine_title(),
      body: m.about_flow_refine_body(),
      layer: 'Silver · Gold'
    },
    { title: m.about_flow_serve_title(), body: m.about_flow_serve_body(), layer: null },
    { title: m.about_flow_explore_title(), body: m.about_flow_explore_body(), layer: null }
  ]);
</script>

<figure class="flow">
  <ol class="flow-steps">
    {#each steps as step, i (i)}
      <li class="flow-step">
        <div class="flow-rail">
          <span class="flow-node">{i + 1}</span>
        </div>
        <div class="flow-card">
          <div class="flow-card-head">
            <h4 class="flow-step-title">{step.title}</h4>
            {#if step.layer}
              <span class="flow-layer">
                <span class="flow-layer-label">{m.about_flow_layer_label()}</span>
                {step.layer}
              </span>
            {/if}
          </div>
          <p class="flow-step-body">{step.body}</p>
        </div>
      </li>
    {/each}
  </ol>
  <figcaption class="flow-caption">{m.about_flow_caption()}</figcaption>
</figure>

<style>
  .flow {
    margin: 0;
  }

  .flow-steps {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
  }

  .flow-step {
    display: grid;
    grid-template-columns: 2rem 1fr;
    column-gap: var(--space-4);
  }

  /* The numbered spine. A connector runs from each node down to the next; the
     last step has none. The node sits on top with an opaque fill so the line
     reads as flowing "through" it. */
  .flow-rail {
    position: relative;
    display: flex;
    justify-content: center;
  }
  .flow-step:not(:last-child) .flow-rail::after {
    content: '';
    position: absolute;
    top: 2rem;
    bottom: 0;
    left: 50%;
    width: 2px;
    transform: translateX(-50%);
    background: linear-gradient(
      to bottom,
      var(--color-accent),
      color-mix(in srgb, var(--color-accent) 35%, transparent)
    );
  }

  .flow-node {
    position: relative;
    z-index: 1;
    width: 2rem;
    height: 2rem;
    flex: none;
    border-radius: 50%;
    display: grid;
    place-items: center;
    background: var(--color-bg-elevated);
    border: 1.5px solid var(--color-accent);
    color: var(--color-accent);
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-semibold);
  }

  .flow-card {
    /* Spacing between steps lives here so the rail connector (anchored to the
       grid row) spans it and reaches the next node. */
    padding: 0 0 var(--space-5);
    min-width: 0;
  }
  .flow-step:last-child .flow-card {
    padding-bottom: 0;
  }

  .flow-card-head {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
    /* Optically centre the title against the 2rem node. */
    min-height: 2rem;
    margin-bottom: var(--space-1);
  }

  .flow-step-title {
    margin: 0;
    font-size: var(--font-size-base);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    line-height: 1.3;
  }

  /* Medallion layer tag — a quiet pill, accent-tinted, so the two storage steps
     read as "this is where the evidence is kept". */
  .flow-layer {
    display: inline-flex;
    align-items: baseline;
    gap: var(--space-1);
    padding: 1px var(--space-2);
    border-radius: var(--radius-pill);
    background: color-mix(in srgb, var(--color-accent) 12%, transparent);
    border: 1px solid color-mix(in srgb, var(--color-accent) 40%, transparent);
    color: var(--color-accent);
    font-family: var(--font-mono);
    font-size: 10.5px;
    font-weight: var(--font-weight-semibold);
    letter-spacing: 0.04em;
    white-space: nowrap;
  }
  .flow-layer-label {
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: color-mix(in srgb, var(--color-accent) 70%, var(--color-fg-subtle));
    font-size: 9px;
  }

  .flow-step-body {
    margin: 0;
    font-size: var(--font-size-sm);
    line-height: 1.55;
    color: var(--color-fg-muted);
  }

  .flow-caption {
    margin: var(--space-4) 0 0;
    padding: var(--space-3) var(--space-4);
    border-left: 2px solid color-mix(in srgb, var(--color-accent) 45%, var(--color-border));
    background: color-mix(in srgb, var(--color-accent) 5%, transparent);
    border-radius: var(--radius-sm);
    font-size: var(--font-size-xs);
    line-height: 1.55;
    color: var(--color-fg-muted);
  }
</style>
