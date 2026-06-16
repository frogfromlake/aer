<script lang="ts">
  // Phase 141 — the saved-analyses list table, extracted from AnalysesOverlay.
  // Pure presentation: it renders the loading / error / empty / populated
  // states of the (already filtered + sorted) rows, the sortable column header,
  // and the per-row owner cell + access badge. All state lives in the parent;
  // this child only emits row-open and sort-toggle intents.
  import * as api from '$lib/api/analyses';
  import AuthNotice from '$lib/components/auth/AuthNotice.svelte';
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
    <p class="muted pad">Loading…</p>
  {:else if loadError}
    <AuthNotice variant="error">{loadError}</AuthNotice>
  {:else if rows.length === 0}
    <p class="muted pad">
      {totalCount === 0
        ? 'No saved analyses yet. Open the Workbench and use “Save current view”.'
        : 'No analyses match these filters.'}
    </p>
  {:else}
    <table>
      <thead>
        <tr>
          <th
            ><button type="button" onclick={() => onToggleSort('name')}
              >Name {sortArrow(sortKey, sortDir, 'name')}</button
            ></th
          >
          <th class="hide-sm">Description</th>
          <th
            ><button type="button" onclick={() => onToggleSort('ownerEmail')}
              >Owner {sortArrow(sortKey, sortDir, 'ownerEmail')}</button
            ></th
          >
          <th
            ><button type="button" onclick={() => onToggleSort('createdAt')}
              >Created {sortArrow(sortKey, sortDir, 'createdAt')}</button
            ></th
          >
          <th
            ><button type="button" onclick={() => onToggleSort('updatedAt')}
              >Updated {sortArrow(sortKey, sortDir, 'updatedAt')}</button
            ></th
          >
          <th>Access</th>
        </tr>
      </thead>
      <tbody>
        {#each rows as a (a.id)}
          <tr
            class:selected={a.id === selectedId}
            onclick={() => onOpenRow(a)}
            aria-label="Open {a.name}"
          >
            <td class="name">{a.name}</td>
            <td class="hide-sm desc">{a.description || '—'}</td>
            <td>{a.owned ? 'You' : a.ownerEmail}</td>
            <td>{fmtDate(a.createdAt)}</td>
            <td>{fmtDate(a.updatedAt)}</td>
            <td>
              <span class="badge" class:own={a.owned} class:edit={a.permission === 'editable'}>
                {a.owned ? 'Owner' : a.permission === 'editable' ? 'Editable' : 'Read-only'}
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
