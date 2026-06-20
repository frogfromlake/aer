<script lang="ts">
  // Task C — the "metadata fields" section on /reflection/metrics. Explains the
  // Tier-B / Tier-C system and, for every tracked field, shows its corpus-wide
  // fill status LIVE (GET /metadata-fields). The central honest point: a field
  // is empty because the publisher does not declare it (structural absence — a
  // publisher choice, WP-003 §3.2), NOT because AĒR lacks an extractor — AĒR
  // attempts extraction for all 22 from structured metadata. The few that are
  // present-but-constant carry no signal (the Task-A condition at corpus scale).
  import { createQuery } from '@tanstack/svelte-query';
  import {
    metadataFieldsQuery,
    type MetadataFieldsResponseDto,
    type MetadataFieldStatDto,
    type QueryOutcome,
    type FetchContext
  } from '$lib/api/queries';
  import {
    TIER_B_FIELDS,
    TIER_C_FIELDS,
    classifyFieldStatus,
    populationPct,
    type FieldStatus
  } from '$lib/reflection/metadata-fields';
  import { fieldLabel } from '$lib/state/labels.svelte';
  import { m } from '$lib/paraglide/messages.js';

  const ctx: FetchContext = { baseUrl: '/api/v1' };

  const fieldsQ = createQuery<
    QueryOutcome<MetadataFieldsResponseDto>,
    Error,
    QueryOutcome<MetadataFieldsResponseDto>
  >(() => {
    const o = metadataFieldsQuery(ctx);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  const byField = $derived<Record<string, MetadataFieldStatDto>>(
    fieldsQ.data?.kind === 'success'
      ? Object.fromEntries(fieldsQ.data.data.fields.map((f) => [f.field, f]))
      : {}
  );
  const failed = $derived(!fieldsQ.isPending && fieldsQ.data?.kind !== 'success');

  // Per-field one-line description (curated). Keys mirror the field names.
  const FIELD_DESC: Record<string, () => string> = {
    published_date: m.metadata_field_desc_published_date,
    modified_date: m.metadata_field_desc_modified_date,
    author: m.metadata_field_desc_author,
    description: m.metadata_field_desc_description,
    categories: m.metadata_field_desc_categories,
    tags: m.metadata_field_desc_tags,
    section: m.metadata_field_desc_section,
    image_url: m.metadata_field_desc_image_url,
    article_type: m.metadata_field_desc_article_type,
    word_count: m.metadata_field_desc_word_count,
    comment_count: m.metadata_field_desc_comment_count,
    comment_url: m.metadata_field_desc_comment_url,
    editor: m.metadata_field_desc_editor,
    reading_time_minutes: m.metadata_field_desc_reading_time_minutes,
    dateline_location: m.metadata_field_desc_dateline_location,
    paywall_status: m.metadata_field_desc_paywall_status,
    correction_notice: m.metadata_field_desc_correction_notice,
    editorial_labels: m.metadata_field_desc_editorial_labels,
    external_citations: m.metadata_field_desc_external_citations,
    images: m.metadata_field_desc_images,
    social_share_counts: m.metadata_field_desc_social_share_counts,
    revision_date: m.metadata_field_desc_revision_date
  };

  function describe(field: string): string {
    const fn = FIELD_DESC[field];
    return fn ? fn() : field;
  }

  function statusLabel(status: FieldStatus): string {
    switch (status) {
      case 'populated':
        return m.metadata_field_status_populated();
      case 'partial':
        return m.metadata_field_status_partial();
      case 'constant':
        return m.metadata_field_status_constant();
      case 'absent':
        return m.metadata_field_status_absent();
      default:
        return m.metadata_field_status_unobserved();
    }
  }

  const tiers = [
    {
      tier: 'B' as const,
      label: m.metadata_tier_b_label,
      blurb: m.metadata_tier_b_blurb,
      fields: TIER_B_FIELDS
    },
    {
      tier: 'C' as const,
      label: m.metadata_tier_c_label,
      blurb: m.metadata_tier_c_blurb,
      fields: TIER_C_FIELDS
    }
  ];
</script>

<section class="mf" id="metadata-fields" aria-labelledby="metadata-fields-heading">
  <header class="mf-header">
    <p class="mf-eyebrow">{m.metadata_fields_eyebrow()}</p>
    <h2 id="metadata-fields-heading" class="mf-h2">{m.metadata_fields_heading()}</h2>
    <p class="mf-lede">{m.metadata_fields_intro()}</p>
    <p class="mf-note">{m.metadata_fields_why_empty()}</p>
  </header>

  {#if fieldsQ.isPending}
    <p class="mf-state" aria-busy="true">{m.metadata_fields_loading()}</p>
  {:else if failed}
    <p class="mf-state">{m.metadata_fields_error()}</p>
  {:else}
    {#each tiers as t (t.tier)}
      <div class="mf-tier">
        <div class="mf-tier-head">
          <span class="mf-tier-badge mf-tier-{t.tier}">{t.label()}</span>
          <p class="mf-tier-blurb">{t.blurb()}</p>
        </div>
        <ul class="mf-list" role="list">
          {#each t.fields as field (field)}
            {@const stat = byField[field]}
            {@const status = classifyFieldStatus(stat)}
            <li class="mf-field mf-status-{status}">
              <div class="mf-field-top">
                <span class="mf-field-label">{fieldLabel(field)}</span>
                <code class="mf-field-name" title={field}>{field}</code>
                <span class="mf-badge mf-badge-{status}" title={describe(field)}>
                  {statusLabel(status)}
                </span>
              </div>
              <p class="mf-desc">{describe(field)}</p>
              <p class="mf-meta">
                {#if status === 'unobserved'}
                  {m.metadata_field_meta_unobserved()}
                {:else if status === 'constant'}
                  {m.metadata_field_meta_constant({
                    value: stat?.constantValue ?? '',
                    sources: stat?.sourcesPopulated ?? 0
                  })}
                {:else}
                  {m.metadata_field_meta_fill({
                    pct: populationPct(stat),
                    populated: stat?.sourcesPopulated ?? 0,
                    observed: stat?.sourcesObserved ?? 0
                  })}
                {/if}
              </p>
            </li>
          {/each}
        </ul>
      </div>
    {/each}
  {/if}
</section>

<style>
  .mf {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    padding-top: var(--space-5);
    border-top: 1px solid var(--color-border);
  }

  .mf-header {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .mf-eyebrow {
    font-size: var(--font-size-xs);
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    margin: 0;
    font-family: var(--font-mono);
  }

  .mf-h2 {
    font-size: var(--font-size-xl);
    font-weight: var(--font-weight-semibold);
    letter-spacing: var(--letter-spacing-tight);
    color: var(--color-fg);
    margin: 0;
    line-height: var(--line-height-tight);
  }

  .mf-lede,
  .mf-note {
    font-size: var(--font-size-base);
    line-height: var(--line-height-loose);
    color: var(--color-fg-muted);
    margin: 0;
  }

  .mf-note {
    font-size: var(--font-size-sm);
    color: var(--color-fg-subtle);
  }

  .mf-state {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  .mf-tier {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .mf-tier-head {
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
    flex-wrap: wrap;
  }

  .mf-tier-badge {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    font-weight: var(--font-weight-semibold);
    padding: 1px 8px;
    border-radius: var(--radius-pill);
    border: 1px solid var(--color-border-strong);
    color: var(--color-fg);
    flex-shrink: 0;
  }

  .mf-tier-blurb {
    margin: 0;
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
  }

  .mf-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(18rem, 1fr));
    gap: var(--space-2);
  }

  .mf-field {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: var(--color-surface);
  }

  .mf-field-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
  }

  .mf-field-label {
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-medium);
    color: var(--color-fg);
  }

  /* Task B — machine name kept as a muted technical reference beside the label. */
  .mf-field-name {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
  }

  .mf-badge {
    margin-left: auto;
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    padding: 1px 6px;
    border-radius: var(--radius-pill);
    border: 1px solid var(--color-border);
    color: var(--color-fg-muted);
    white-space: nowrap;
    flex-shrink: 0;
  }

  /* Populated = the one affirmative state (a calm positive tint); everything
     else is a perceptually-neutral methodological dim (ADR-039
     METHODOLOGICAL-NOT-WARNING — absence/constancy are never errors). */
  .mf-badge-populated {
    color: var(--color-fg);
    border-color: color-mix(
      in srgb,
      var(--color-status-validated, var(--color-fg-subtle)) 50%,
      var(--color-border)
    );
    background: color-mix(
      in srgb,
      var(--color-status-validated, var(--color-fg-subtle)) 8%,
      transparent
    );
  }

  .mf-badge-constant {
    border-color: color-mix(in srgb, var(--color-fg-subtle) 45%, var(--color-border));
    background: color-mix(in srgb, var(--color-fg-subtle) 8%, transparent);
  }

  .mf-desc {
    margin: 0;
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    line-height: var(--line-height-normal);
  }

  .mf-meta {
    margin: 0;
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--color-fg-subtle);
  }
</style>
