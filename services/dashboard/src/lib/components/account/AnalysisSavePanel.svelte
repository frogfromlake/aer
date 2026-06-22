<script lang="ts">
  // The "Save current view" panel of the saved-analyses overlay (Phase 148e).
  // Extracted from AnalysesOverlay to keep that orchestrator under the file-
  // length ratchet and to give the save action its own titled block, distinct
  // from the list/search below. The parent owns all state + the API calls; this
  // is presentational and reports intent through callbacks.
  import Button from '$lib/components/base/Button.svelte';
  import { m } from '$lib/paraglide/messages.js';
  import type { AnalysisListItem } from '$lib/api/analyses';

  interface Props {
    /** Show the update-or-new choice (an editable analysis is loaded). */
    showChoice: boolean;
    loadedAnalysis: AnalysisListItem | null;
    saveName: string;
    saveDescription: string;
    saving: boolean;
    onUpdate: () => void;
    onSaveAsNewChoice: () => void;
    onSubmit: (event: SubmitEvent) => void;
    onCancel: () => void;
  }
  let {
    showChoice,
    loadedAnalysis,
    saveName = $bindable(),
    saveDescription = $bindable(),
    saving,
    onUpdate,
    onSaveAsNewChoice,
    onSubmit,
    onCancel
  }: Props = $props();
</script>

<div class="save-form">
  <h3 class="save-title">{m.account_analyses_save_current()}</h3>
  {#if showChoice && loadedAnalysis}
    <p class="muted">{m.account_analyses_save_choice({ name: loadedAnalysis.name })}</p>
    <div class="row-actions">
      <Button variant="primary" loading={saving} onclick={onUpdate}
        >{m.account_analyses_save_update({ name: loadedAnalysis.name })}</Button
      >
      <Button variant="secondary" onclick={onSaveAsNewChoice}
        >{m.account_analyses_save_as_new()}</Button
      >
      <Button variant="secondary" onclick={onCancel}>{m.common_cancel()}</Button>
    </div>
  {:else}
    <p class="muted">{m.account_analyses_save_intro()}</p>
    <form class="save-fields" onsubmit={onSubmit} novalidate>
      <input
        class="field"
        placeholder={m.account_analyses_save_name_placeholder()}
        bind:value={saveName}
        aria-label={m.account_analyses_save_name_label()}
      />
      <input
        class="field"
        placeholder={m.account_analyses_save_description_placeholder()}
        bind:value={saveDescription}
        aria-label={m.account_analyses_save_description_label()}
      />
      <div class="row-actions">
        <Button type="submit" variant="primary" loading={saving} disabled={!saveName.trim()}
          >{m.common_save()}</Button
        >
        <Button variant="secondary" onclick={onCancel}>{m.common_cancel()}</Button>
      </div>
    </form>
  {/if}
</div>

<style>
  .save-form {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-3);
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }
  .save-title {
    margin: 0 0 var(--space-1);
    font-size: var(--font-size-base);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
  }
  .save-fields {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .field {
    width: 100%;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-fg);
    padding: var(--space-2) var(--space-3);
    font-size: var(--font-size-sm);
    font-family: var(--font-ui);
  }
  .field:focus-visible {
    outline: none;
    border-color: var(--color-accent);
    box-shadow: 0 0 0 var(--focus-ring-width)
      color-mix(in oklab, var(--color-accent) 40%, transparent);
  }
  .row-actions {
    display: flex;
    gap: var(--space-2);
    flex-wrap: wrap;
  }
  .muted {
    color: var(--color-fg-muted);
    font-size: var(--font-size-sm);
    margin: 0;
    line-height: var(--line-height-base);
  }
</style>
