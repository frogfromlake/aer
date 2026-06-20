<script lang="ts">
  // WorkbenchLeaveGuard — Phase 127.
  //
  // Confirm modal shown when the user navigates away from the Workbench with a
  // CHANGED, unsaved analysis. Three exits, mirroring the saved-analysis model:
  //   · leave without saving        (discard the in-flight changes)
  //   · save as a new analysis      (name → createAnalysis → leave)
  //   · update the loaded analysis  (only when one was opened → updateAnalysis)
  // The analysis state is the URL deep-link (overlay params already stripped by
  // the page). On a successful save the leave-guard baseline is reset to clean
  // so the now-saved view leaves silently next time.
  import Dialog from '$lib/components/base/Dialog.svelte';
  import Button from '$lib/components/base/Button.svelte';
  import * as api from '$lib/api/analyses';
  import { setCleanBaseline } from '$lib/workbench/dirty.svelte';
  import { m } from '$lib/paraglide/messages.js';

  interface Props {
    open: boolean;
    /** The saved analysis the current view was opened from, or null. */
    loadedAnalysisId: string | null;
    /** The deep-link to persist (computed by the page when the guard opens). */
    currentState: string;
    /** Proceed with the intercepted navigation. */
    onLeave: () => void;
    /** Cancel — stay on the Workbench. */
    onCancel: () => void;
  }
  let { open, loadedAnalysisId, currentState, onLeave, onCancel }: Props = $props();

  let mode = $state<'choices' | 'name'>('choices');
  let name = $state('');
  let description = $state('');
  let saving = $state(false);
  let error = $state<string | null>(null);

  // Reset the transient panel each time the modal opens.
  $effect(() => {
    if (open) {
      mode = 'choices';
      name = '';
      description = '';
      saving = false;
      error = null;
    }
  });

  async function saveNewAndLeave(event: SubmitEvent) {
    event.preventDefault();
    if (!name.trim() || saving) return;
    saving = true;
    error = null;
    const res = await api.createAnalysis(name.trim(), description.trim(), currentState);
    saving = false;
    if (res.ok) {
      setCleanBaseline(currentState);
      onLeave();
    } else {
      error = res.message || m.workbench_leave_save_failed();
    }
  }

  async function updateAndLeave() {
    if (!loadedAnalysisId || saving) return;
    saving = true;
    error = null;
    const res = await api.updateAnalysis(loadedAnalysisId, { state: currentState });
    saving = false;
    if (res.ok) {
      setCleanBaseline(currentState);
      onLeave();
    } else {
      error = res.message || m.workbench_leave_save_failed();
    }
  }
</script>

<Dialog {open} title={m.workbench_leave_title()} onClose={onCancel}>
  {#if mode === 'choices'}
    <p class="lg-body">{m.workbench_leave_body()}</p>
    {#if error}<p class="lg-error" role="alert">{error}</p>{/if}
    <div class="lg-actions">
      <Button variant="primary" onclick={() => (mode = 'name')}>
        {m.workbench_leave_save_new()}
      </Button>
      {#if loadedAnalysisId}
        <Button variant="secondary" loading={saving} onclick={updateAndLeave}>
          {m.workbench_leave_update()}
        </Button>
      {/if}
      <Button variant="ghost" onclick={onLeave}>{m.workbench_leave_discard()}</Button>
    </div>
    <div class="lg-stay">
      <button type="button" class="lg-stay-btn" onclick={onCancel}>
        {m.workbench_leave_cancel()}
      </button>
    </div>
  {:else}
    <form onsubmit={saveNewAndLeave}>
      <label class="lg-label" for="lg-name">{m.workbench_leave_name_label()}</label>
      <input
        id="lg-name"
        class="lg-input"
        bind:value={name}
        placeholder={m.workbench_leave_name_placeholder()}
        autocomplete="off"
        maxlength="120"
      />
      {#if error}<p class="lg-error" role="alert">{error}</p>{/if}
      <div class="lg-actions">
        <Button type="submit" variant="primary" loading={saving} disabled={!name.trim()}>
          {m.workbench_leave_save_new_submit()}
        </Button>
        <Button type="button" variant="ghost" onclick={() => (mode = 'choices')}>
          {m.workbench_leave_back()}
        </Button>
      </div>
    </form>
  {/if}
</Dialog>

<style>
  .lg-body {
    margin: 0 0 var(--space-3);
    color: var(--color-fg-muted);
    font-size: var(--font-size-sm);
  }
  .lg-actions {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    margin-top: var(--space-3);
  }
  .lg-stay {
    margin-top: var(--space-3);
    text-align: center;
  }
  .lg-stay-btn {
    appearance: none;
    background: none;
    border: none;
    color: var(--color-fg-subtle);
    font-size: var(--font-size-xs);
    text-decoration: underline;
    cursor: pointer;
  }
  .lg-stay-btn:hover,
  .lg-stay-btn:focus-visible {
    color: var(--color-fg);
  }
  .lg-label {
    display: block;
    margin-bottom: var(--space-2);
    font-size: var(--font-size-sm);
    color: var(--color-fg);
  }
  .lg-input {
    width: 100%;
    padding: var(--space-2) var(--space-3);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg);
    font-size: var(--font-size-sm);
  }
  .lg-error {
    margin: var(--space-2) 0 0;
    color: var(--color-danger, #e5484d);
    font-size: var(--font-size-xs);
  }
</style>
