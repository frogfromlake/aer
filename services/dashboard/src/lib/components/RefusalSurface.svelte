<script lang="ts">
  // Refusal surface primitive (Design Brief §5.4).
  //
  // When the BFF returns a methodological-gate 400 (validation missing,
  // equivalence missing, k-anonymity threshold not met), the frontend
  // must not render it as a generic error. It renders as a statement of
  // *why* the system declined to answer — a register-paired block
  // authored under `configs/content/refusals/` that explains the gate
  // in both semantic and methodological registers.
  //
  // This component is the first end-to-end consumer of the Content
  // Catalog. It takes a `RefusalOutcome` from the query layer, looks up
  // the authored content for that refusal kind, and renders it through
  // `ProgressiveSemantics`. The raw BFF message is only shown when the
  // Content Catalog lookup itself fails — we don't want an authored-text
  // regression to swallow the underlying refusal reason.
  import { createQuery } from '@tanstack/svelte-query';
  import {
    contentQuery,
    type ContentResponseDto,
    type FetchContext,
    type QueryOutcome,
    type RefusalOutcome
  } from '$lib/api/queries';
  import ProgressiveSemantics from './ProgressiveSemantics.svelte';

  interface Props {
    refusal: RefusalOutcome;
    ctx: FetchContext;
    locale?: 'en' | 'de';
  }

  let { refusal, ctx, locale = 'en' }: Props = $props();

  // `unspecified` has no authored content — fall back to the raw message.
  // For any authored kind we query `/content/refusal/{kind}`.
  const query = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const options = contentQuery(ctx, 'refusal', refusal.refusalKind, locale);
    return {
      queryKey: [...options.queryKey],
      queryFn: options.queryFn,
      staleTime: options.staleTime,
      enabled: refusal.refusalKind !== 'unspecified'
    };
  });
</script>

<section class="refusal" role="status" aria-live="polite">
  <header>
    <span class="badge" aria-label="System declined to answer">Refusal</span>
    <span class="kind">{refusal.refusalKind.replaceAll('_', ' ')}</span>
  </header>

  {#if refusal.refusalKind === 'unspecified'}
    <p class="raw">{refusal.message}</p>
  {:else if query.isPending}
    <p class="muted" aria-busy="true">Loading methodological register…</p>
  {:else if query.isError}
    <p class="raw">{refusal.message}</p>
    <p class="muted">Content Catalog unavailable.</p>
  {:else if query.data?.kind === 'success'}
    <ProgressiveSemantics registers={query.data.data.registers} emphasis="methodological" />
  {:else}
    <!-- Catalog itself returned a refusal or a non-success outcome: show the raw BFF message. -->
    <p class="raw">{refusal.message}</p>
  {/if}
</section>

<style>
  .refusal {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    padding: var(--space-4);
    border: 1px solid var(--color-border);
    border-left: 3px solid var(--color-accent);
    border-radius: var(--radius-md);
    background: var(--color-surface);
  }
  header {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .badge {
    display: inline-block;
    padding: 2px var(--space-2);
    border-radius: var(--radius-sm);
    background: var(--color-accent);
    color: var(--color-bg);
    font-size: var(--font-size-xs);
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }
  .kind {
    font-family: var(--font-family-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
  }
  .raw {
    margin: 0;
    color: var(--color-fg);
    font-size: var(--font-size-base);
    line-height: 1.55;
  }
  .muted {
    margin: 0;
    color: var(--color-fg-muted);
    font-size: var(--font-size-sm);
  }
</style>
