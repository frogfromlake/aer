<script lang="ts">
  // Phase 141 — the saved-analyses list table, extracted from AnalysesOverlay.
  // Phase 148e — resizable + reorderable, persisted columns: each row stays a
  // single line (cells truncate within their column width), a column edge drags
  // to resize, a column header drags to reorder, both persist to localStorage,
  // and the table scrolls horizontally when it outgrows the panel (Shift +
  // wheel). The render is column-order-driven; all DATA state lives in the
  // parent, this child owns only the column layout UI state.
  import * as api from '$lib/api/analyses';
  import AuthNotice from '$lib/components/auth/AuthNotice.svelte';
  import { m } from '$lib/paraglide/messages.js';
  import { locale } from '$lib/state/locale.svelte';
  import { sortArrow, fmtDate, type SortKey, type SortDir } from './analyses-overlay-internals';
  import {
    loadColumnWidths,
    saveColumnWidths,
    resetColumnWidths,
    clampColumnWidth,
    totalColumnsWidth,
    loadColumnOrder,
    saveColumnOrder,
    resetColumnOrder,
    moveColumn,
    type AnalysisColumnId,
    type ColumnWidths
  } from './analyses-table-columns';

  // Per-column header label + (optional) sort key. The render iterates `order`,
  // so these maps keep the dynamic per-id lookup in one place.
  const HEADER_LABEL: Record<AnalysisColumnId, () => string> = {
    name: m.account_analyses_col_name,
    description: m.account_analyses_col_description,
    owner: m.account_analyses_col_owner,
    created: m.account_analyses_col_created,
    updated: m.account_analyses_col_updated,
    access: m.account_analyses_col_access
  };
  const SORT_KEY: Partial<Record<AnalysisColumnId, SortKey>> = {
    name: 'name',
    owner: 'ownerEmail',
    created: 'createdAt',
    updated: 'updatedAt'
  };

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

  // --- resizable columns (persisted) ----------------------------------------
  let widths = $state<ColumnWidths>(loadColumnWidths());
  let order = $state<AnalysisColumnId[]>(loadColumnOrder());
  const tableWidth = $derived(totalColumnsWidth(widths));

  let resize = $state<{ id: AnalysisColumnId; startX: number; startW: number } | null>(null);

  function onResizeStart(e: PointerEvent, id: AnalysisColumnId) {
    e.preventDefault();
    e.stopPropagation();
    resize = { id, startX: e.clientX, startW: widths[id] };
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
  }
  function onResizeMove(e: PointerEvent) {
    if (!resize) return;
    widths = {
      ...widths,
      [resize.id]: clampColumnWidth(resize.startW + (e.clientX - resize.startX))
    };
  }
  function onResizeEnd() {
    if (!resize) return;
    resize = null;
    saveColumnWidths(widths);
  }

  // --- reorderable columns (HTML5 drag on the header; persisted) -------------
  let draggingCol = $state<AnalysisColumnId | null>(null);
  let overCol = $state<AnalysisColumnId | null>(null);

  function onColDragStart(e: DragEvent, id: AnalysisColumnId) {
    draggingCol = id;
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = 'move';
      e.dataTransfer.setData('text/plain', id); // Firefox needs data to drag
    }
  }
  function onColDragOver(e: DragEvent, id: AnalysisColumnId) {
    if (!draggingCol) return;
    e.preventDefault(); // allow drop
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
    overCol = id;
  }
  function onColDrop(e: DragEvent, id: AnalysisColumnId) {
    e.preventDefault();
    if (draggingCol) {
      order = moveColumn(order, draggingCol, id);
      saveColumnOrder(order);
    }
    draggingCol = null;
    overCol = null;
  }
  function onColDragEnd() {
    draggingCol = null;
    overCol = null;
  }

  function onResetColumns() {
    widths = resetColumnWidths();
    order = resetColumnOrder();
  }

  // The shift-scroll hint shows ONLY when the table actually overflows its
  // viewport horizontally — no clutter when everything fits. Re-measure when the
  // column widths change (a resize) and when the container resizes.
  let wrapEl = $state<HTMLDivElement | null>(null);
  let scrollable = $state(false);

  function measureScrollable() {
    if (wrapEl) scrollable = wrapEl.scrollWidth > wrapEl.clientWidth + 1;
  }
  $effect(() => {
    void tableWidth; // re-run after a column resize reflows the table
    measureScrollable();
  });
  $effect(() => {
    if (!wrapEl || typeof ResizeObserver === 'undefined') return;
    const ro = new ResizeObserver(() => measureScrollable());
    ro.observe(wrapEl);
    return () => ro.disconnect();
  });
</script>

{#snippet resizeHandle(id: AnalysisColumnId)}
  <!-- draggable=false so grabbing the edge resizes (pointer events) rather than
       starting the column reorder drag on the header. -->
  <span
    class="col-resize"
    aria-hidden="true"
    draggable="false"
    onpointerdown={(e) => onResizeStart(e, id)}
    onpointermove={onResizeMove}
    onpointerup={onResizeEnd}
    onpointercancel={onResizeEnd}
  ></span>
{/snippet}

<div class="table-host" class:resizing={resize !== null} class:reordering={draggingCol !== null}>
  {#if loading}
    <p class="muted pad">{m.common_loading()}</p>
  {:else if loadError}
    <AuthNotice variant="error">{loadError}</AuthNotice>
  {:else if rows.length === 0}
    <p class="muted pad">
      {totalCount === 0 ? m.account_analyses_empty_none() : m.account_analyses_empty_filtered()}
    </p>
  {:else}
    <div class="table-tools">
      <button type="button" class="reset-cols" onclick={onResetColumns}>
        {m.account_analyses_reset_columns()}
      </button>
    </div>
    <div class="table-wrap" bind:this={wrapEl}>
      <table style="width: {tableWidth}px">
        <thead>
          <tr>
            {#each order as id (id)}
              <th
                style="width: {widths[id]}px"
                class:dragging={draggingCol === id}
                class:drag-over={overCol === id && draggingCol !== id}
                draggable="true"
                ondragstart={(e) => onColDragStart(e, id)}
                ondragover={(e) => onColDragOver(e, id)}
                ondragleave={() => (overCol = overCol === id ? null : overCol)}
                ondrop={(e) => onColDrop(e, id)}
                ondragend={onColDragEnd}
              >
                {#if SORT_KEY[id]}
                  <button type="button" class="th-label" onclick={() => onToggleSort(SORT_KEY[id]!)}
                    >{HEADER_LABEL[id]()} {sortArrow(sortKey, sortDir, SORT_KEY[id]!)}</button
                  >
                {:else}
                  <span class="th-label">{HEADER_LABEL[id]()}</span>
                {/if}
                {@render resizeHandle(id)}
              </th>
            {/each}
          </tr>
        </thead>
        <tbody>
          {#each rows as a (a.id)}
            <tr
              class:selected={a.id === selectedId}
              onclick={() => onOpenRow(a)}
              aria-label={m.account_analyses_open_row({ name: a.name })}
            >
              {#each order as id (id)}
                {#if id === 'name'}
                  <td class="name" title={a.name}>{a.name}</td>
                {:else if id === 'description'}
                  <td class="desc" title={a.description || undefined}>{a.description || '—'}</td>
                {:else if id === 'owner'}
                  <td title={a.owned ? undefined : a.ownerEmail}>
                    {a.owned ? m.account_analyses_owner_you() : a.ownerName}
                  </td>
                {:else if id === 'created'}
                  <td>{fmtDate(a.createdAt, locale())}</td>
                {:else if id === 'updated'}
                  <td>{fmtDate(a.updatedAt, locale())}</td>
                {:else}
                  <td>
                    <span
                      class="badge"
                      class:own={a.owned}
                      class:edit={a.permission === 'editable'}
                    >
                      {a.owned
                        ? m.account_analyses_access_owner()
                        : a.permission === 'editable'
                          ? m.account_analyses_access_editable()
                          : m.account_analyses_access_readonly()}
                    </span>
                  </td>
                {/if}
              {/each}
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
    {#if scrollable}
      <div class="scroll-hint" role="note">
        <span aria-hidden="true">↔</span>
        {m.account_analyses_scroll_hint()}
      </div>
    {/if}
  {/if}
</div>

<style>
  .table-host {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    /* min-width:0 lets this flex child shrink below the table's width so the
       inner scroller can clip + scroll it (Phase 148e fix). */
    min-width: 0;
  }
  /* While resizing or reordering a column, suppress text selection so the drag
     feels solid; resizing also forces the resize cursor everywhere. */
  .table-host.resizing,
  .table-host.reordering {
    user-select: none;
  }
  .table-host.resizing {
    cursor: col-resize;
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

  /* Slim tools row — the "reset columns" affordance, right-aligned. */
  .table-tools {
    display: flex;
    justify-content: flex-end;
    padding: 0 var(--space-1) var(--space-2);
  }
  /* Shift-scroll hint — a quiet, dimmed centred chip BELOW the table (under the
     horizontal scrollbar), shown only while the table actually overflows. */
  .scroll-hint {
    align-self: center;
    flex-shrink: 0;
    margin-top: var(--space-2);
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: var(--space-1) var(--space-3);
    background: color-mix(in srgb, var(--color-bg-elevated) 60%, transparent);
    border: 1px solid color-mix(in srgb, var(--color-border) 60%, transparent);
    border-radius: var(--radius-pill);
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    white-space: nowrap;
    animation: scroll-hint-in var(--motion-duration-fast) var(--motion-ease-standard);
  }
  @keyframes scroll-hint-in {
    from {
      opacity: 0;
      transform: translateY(3px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .scroll-hint {
      animation: none;
    }
  }
  .reset-cols {
    appearance: none;
    background: none;
    border: none;
    color: var(--color-fg-muted);
    font-size: var(--font-size-xs);
    cursor: pointer;
    padding: 0;
  }
  .reset-cols:hover,
  .reset-cols:focus-visible {
    color: var(--color-fg);
    text-decoration: underline;
    outline: none;
  }

  .table-wrap {
    flex: 1;
    /* Both axes: rows never grow in height (cells truncate); a too-wide table
       scrolls horizontally. Shift + wheel scrolls sideways natively. */
    overflow: auto;
    min-height: 0;
    min-width: 0;
  }
  table {
    /* Fixed layout so the per-column widths are exact and content truncates
       within them; width = sum of columns (set inline) so resizing is 1:1 and
       the table scrolls rather than redistributing slack. */
    table-layout: fixed;
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
    overflow: hidden;
    text-overflow: ellipsis;
    /* The header is draggable to reorder columns. */
    cursor: grab;
  }
  thead th.dragging {
    opacity: 0.45;
  }
  /* Drop-target indicator — a left accent edge where the column will land. */
  thead th.drag-over {
    box-shadow: inset 2px 0 0 var(--color-accent);
  }
  .table-host.reordering thead th {
    cursor: grabbing;
  }
  /* The sort affordance (a real button) keeps the pointer cursor + truncates. */
  .th-label {
    display: inline-block;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    vertical-align: bottom;
  }
  button.th-label {
    background: none;
    border: none;
    color: inherit;
    font: inherit;
    cursor: pointer;
    padding: 0;
  }
  button.th-label:hover {
    color: var(--color-fg);
  }
  /* Column-resize grip on the right edge of each header cell. The sticky header
     is a containing block for this absolutely-positioned handle. */
  .col-resize {
    position: absolute;
    top: 0;
    right: 0;
    width: 9px;
    height: 100%;
    cursor: col-resize;
    touch-action: none;
  }
  .col-resize::after {
    content: '';
    position: absolute;
    top: 25%;
    bottom: 25%;
    right: 3px;
    width: 1px;
    background: var(--color-border);
  }
  .col-resize:hover::after {
    width: 2px;
    background: var(--color-accent);
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
    /* Single line + truncate within the column width → uniform row height. */
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  td.name {
    font-weight: var(--font-weight-medium);
  }
  td.desc {
    color: var(--color-fg-muted);
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
</style>
