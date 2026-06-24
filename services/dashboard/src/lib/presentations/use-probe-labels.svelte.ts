// Phase 148e — reactive probe-label hook.
//
// Thin wrapper over `probesQuery` that hands a cell a `labelFor(id)` accessor,
// generalising LeadLagCell's inline pattern. Call it once at the top of a cell's
// <script> (createQuery needs the component-init context); the returned closure
// reads the live query result, so labels fill in reactively once `/probes`
// resolves and re-render on a language switch. Probe-scoped cells use it to
// resolve a raw probe id to its display label; source-scoped cells get the
// source name back verbatim (the resolver falls through) — safe to use either
// way, so every cell can route its scope through one path.
//
// `ctx` is passed as a GETTER (`() => ctx`) so the query options closure reads it
// reactively — a positional `ctx` would only capture the initial value (Svelte's
// `state_referenced_locally`) and would not re-fetch if the FetchContext changes
// (e.g. a content-language switch).
import { createQuery } from '@tanstack/svelte-query';
import { probesQuery, type ProbeDto, type QueryOutcome, type FetchContext } from '$lib/api/queries';
import { resolveScopeLabel } from './scope-label';
import { sourceLabel } from '$lib/state/labels.svelte';

export interface ProbeLabels {
  /** Resolve a scope id to its display label; null/empty → an em-dash. */
  labelFor(scopeId: string | null | undefined): string;
}

export function useProbeLabels(getCtx: () => FetchContext): ProbeLabels {
  const q = createQuery<QueryOutcome<ProbeDto[]>, Error, QueryOutcome<ProbeDto[]>>(() => {
    const o = probesQuery(getCtx());
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });
  return {
    labelFor(scopeId: string | null | undefined): string {
      if (!scopeId) return '—';
      const probes = q.data?.kind === 'success' ? q.data.data : [];
      return resolveScopeLabel(scopeId, probes, sourceLabel);
    }
  };
}
