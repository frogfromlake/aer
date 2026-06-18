<script lang="ts">
  // Phase 141 — the saved-analyses list table, extracted from AnalysesOverlay.
  // Pure presentation: it renders the loading / error / empty / populated
  // states of the (already filtered + sorted) rows, the sortable column header,
  // and the per-row owner cell + access badge. All state lives in the parent;
  // this child only emits row-open and sort-toggle intents.
  import * as api from '$lib/api/analyses';
  import AuthNotice from '$lib/components/auth/AuthNotice.svelte';
  import { m } from '$lib/paraglide/messages.js';
  import { locale } from '$lib/state/locale.svelte';
  import { sortArrow, fmtDate, type SortKey, type SortDir } from './analyses-overlay-internals';

  interface Props {
    rows: api.AnalysisListItem[];
    loading: boolean;
    loadError: string | null;
    totalCount: number;
    selectedId: string | null;
    sortKey: SortKey;
    sortDir: SortDir;
    onToggleSort: (key: SortKey) => void;
    onOpenRow: (a: api.AnalysisListItem) => void;
  }

  let {
    rows,
    loading,
    loadError,
    totalCount,
    selectedId,
    sortKey,
    sortDir,
    onToggleSort,
    onOpenRow
  }: Props = $props();
</script>

<div class="table-wrap">
  {#if loading}
    <p class="muted pad">{m.common_loading()}</p>
  {:else if loadError}
    <AuthNotice variant="error">{loadError}</AuthNotice>
  {:else if rows.length === 0}
    <p class="muted pad">
      {totalCount === 0 ? m.account_analyses_empty_none() : m.account_analyses_empty_filtered()}
    </p>
  {:else}
    <table>
      <thead>
        <tr>
          <th
            ><button type="button" onclick={() => onToggleSort('name')}
              >{m.account_analyses_col_name()} {sortArrow(sortKey, sortDir, 'name')}</button
            ></th
          >
          <th class="hide-sm">{m.account_analyses_col_description()}</th>
          <th
            ><button type="button" onclick={() => onToggleSort('ownerEmail')}
              >{m.account_analyses_col_owner()} {sortArrow(sortKey, sortDir, 'ownerEmail')}</button
            ></th
          >
          <th
            ><button type="button" onclick={() => onToggleSort('createdAt')}
              >{m.account_analyses_col_created()} {sortArrow(sortKey, sortDir, 'createdAt')}</button
            ></th
          >
          <th
            ><button type="button" onclick={() => onToggleSort('updatedAt')}
              >{m.account_analyses_col_updated()} {sortArrow(sortKey, sortDir, 'updatedAt')}</button
            ></th
          >
          <th>{m.account_analyses_col_access()}</th>
        </tr>
      </thead>
      <tbody>
        {#each rows as a (a.id)}
          <tr
            class:selected={a.id === selectedId}
            onclick={() => onOpenRow(a)}
            aria-label={m.account_analyses_open_row({ name: a.name })}
          >
            <td class="name">{a.name}</td>
            <td class="hide-sm desc">{a.description || '—'}</td>
            <td>{a.owned ? m.account_analyses_owner_you() : a.ownerEmail}</td>
            <td>{fmtDate(a.createdAt, locale())}</td>
            <td>{fmtDate(a.updatedAt, locale())}</td>
            <td>
              <span class="badge" class:own={a.owned} class:edit={a.permission === 'editable'}>
                {a.owned
                  ? m.account_analyses_access_owner()
                  : a.permission === 'editable'
                    ? m.account_analyses_access_editable()
                    : m.account_analyses_access_readonly()}
              </span>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</div>

<style>
  .table-wrap {
    flex: 1;
    overflow-y: auto;
    min-height: 0;
  }
  .pad {
    padding: var(--space-4);
  }
  .muted {
    color: var(--color-fg-muted);
    font-size: var(--font-size-sm);
    margin: 0;
    line-height: var(--line-height-base);
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--font-size-sm);
  }
  thead th {
    position: sticky;
    top: 0;
    background: var(--color-bg-elevated);
    text-align: left;
    color: var(--color-fg-subtle);
    font-weight: var(--font-weight-medium);
    border-bottom: 1px solid var(--color-border);
    padding: var(--space-2) var(--space-3);
    white-space: nowrap;
  }
  thead th button {
    background: none;
    border: none;
    color: inherit;
    font: inherit;
    cursor: pointer;
    padding: 0;
  }
  thead th button:hover {
    color: var(--color-fg);
  }
  tbody tr {
    cursor: pointer;
    border-bottom: 1px solid var(--color-border);
  }
  tbody tr:hover {
    background: var(--color-surface-hover);
  }
  tbody tr.selected {
    background: var(--color-surface);
  }
  td {
    padding: var(--space-3);
    color: var(--color-fg);
    vertical-align: top;
  }
  td.name {
    font-weight: var(--font-weight-medium);
  }
  td.desc {
    color: var(--color-fg-muted);
    max-width: 18rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .badge {
    font-size: var(--font-size-xs);
    padding: 1px var(--space-2);
    border-radius: var(--radius-pill);
    border: 1px solid var(--color-border);
    color: var(--color-fg-muted);
    white-space: nowrap;
  }
  .badge.edit {
    color: var(--color-accent);
    border-color: var(--color-accent-muted);
  }
  .badge.own {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
  }
  @media (max-width: 720px) {
    .hide-sm {
      display: none;
    }
  }
</style>
