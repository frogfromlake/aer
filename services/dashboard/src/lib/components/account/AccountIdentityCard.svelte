<script lang="ts">
  // Identity card for the account overlay (Phase 148e) — the avatar anchor,
  // name/email, role + colour-coded status chips, and the sign-out affordance
  // kept top-right so it is reachable without scrolling the panel. Extracted
  // from AccountOverlay to keep that file under the file-length ratchet and to
  // make the identity block reusable.
  import type { AuthUser } from '$lib/api/auth';
  import * as authApi from '$lib/api/auth';
  import { doLogout, setUser } from '$lib/state/auth.svelte';
  import UserAvatar from '$lib/components/base/UserAvatar.svelte';
  import AuthField from '$lib/components/auth/AuthField.svelte';
  import Button from '$lib/components/base/Button.svelte';
  import { displayName } from '$lib/identity/initials';
  import { statusLabel } from '$lib/account/status-label';
  import { m } from '$lib/paraglide/messages.js';

  interface Props {
    me: AuthUser | null;
  }
  const { me }: Props = $props();

  // Self-service name edit (Phase 148e). On save the store is refreshed, so the
  // avatar, this card, and the saved-analyses owner column update live.
  let editing = $state(false);
  let editFirst = $state('');
  let editLast = $state('');
  let saving = $state(false);
  let editErr = $state<string | null>(null);

  function startEdit() {
    editFirst = me?.firstName ?? '';
    editLast = me?.lastName ?? '';
    editErr = null;
    editing = true;
  }
  function cancelEdit() {
    editing = false;
    editErr = null;
  }
  async function saveEdit() {
    const fn = editFirst.trim();
    const ln = editLast.trim();
    if (!fn || !ln) {
      editErr = m.account_identity_name_invalid();
      return;
    }
    saving = true;
    editErr = null;
    const res = await authApi.updateProfile(fn, ln);
    saving = false;
    if (res.ok) {
      setUser(res.data);
      editing = false;
    } else {
      editErr = m.account_identity_name_error();
    }
  }
</script>

<section class="block identity-block">
  <div class="id-card">
    <UserAvatar firstName={me?.firstName} lastName={me?.lastName} email={me?.email} size={46} />
    <div class="id-main">
      {#if editing}
        <div class="name-edit">
          <AuthField
            id="id-first"
            compact
            label={m.auth_field_first_name_label()}
            bind:value={editFirst}
            autocomplete="given-name"
            disabled={saving}
          />
          <AuthField
            id="id-last"
            compact
            label={m.auth_field_last_name_label()}
            bind:value={editLast}
            autocomplete="family-name"
            disabled={saving}
          />
        </div>
        {#if editErr}<p class="id-edit-err">{editErr}</p>{/if}
        <div class="id-edit-actions">
          <Button variant="primary" loading={saving} onclick={saveEdit}>{m.common_save()}</Button>
          <button type="button" class="id-edit-cancel" onclick={cancelEdit} disabled={saving}>
            {m.common_cancel()}
          </button>
        </div>
      {:else}
        <p class="id-name">
          {me ? displayName(me) : '—'}
          {#if me}
            <button
              type="button"
              class="id-edit-btn"
              onclick={startEdit}
              aria-label={m.account_identity_edit_name()}
              title={m.account_identity_edit_name()}>✎</button
            >
          {/if}
        </p>
        {#if me && displayName(me) !== me.email}
          <p class="id-email">{me.email}</p>
        {/if}
      {/if}
      <div class="id-chips">
        <span class="chip role">{me?.role ?? '—'}</span>
        <span class="chip status status-{me?.status ?? 'unknown'}">
          <span class="dot" aria-hidden="true"></span>
          {statusLabel(me?.status)}
        </span>
      </div>
    </div>
    <button type="button" class="signout" onclick={() => doLogout()}>
      {m.chrome_user_signout()}
    </button>
  </div>
</section>

<style>
  /* Identity is the first block: no divider above it, it leads the panel. */
  .identity-block {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .id-card {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }
  .id-main {
    flex: 1 1 auto;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .id-name {
    margin: 0;
    font-size: var(--font-size-md);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .id-email {
    margin: 0;
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  /* Self-service name edit. */
  .id-edit-btn {
    appearance: none;
    background: transparent;
    border: none;
    color: var(--color-fg-subtle);
    cursor: pointer;
    font-size: 0.85em;
    padding: 0 var(--space-1);
    line-height: 1;
  }
  .id-edit-btn:hover,
  .id-edit-btn:focus-visible {
    color: var(--color-accent);
    outline: none;
  }
  .name-edit {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-2);
  }
  .id-edit-actions {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    margin-top: var(--space-1);
  }
  .id-edit-cancel {
    appearance: none;
    background: transparent;
    border: none;
    color: var(--color-fg-muted);
    font-size: var(--font-size-sm);
    cursor: pointer;
  }
  .id-edit-cancel:hover:not(:disabled),
  .id-edit-cancel:focus-visible {
    color: var(--color-fg);
    text-decoration: underline;
    outline: none;
  }
  .id-edit-err {
    margin: 0;
    font-size: var(--font-size-xs);
    color: var(--color-status-expired);
  }
  .id-chips {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    margin-top: 3px;
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    padding: 2px 8px;
    border-radius: 999px;
    border: 1px solid var(--color-border);
    color: var(--color-fg-muted);
  }
  .chip.status .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: currentColor;
    flex-shrink: 0;
  }
  .status-active {
    color: var(--color-status-validated);
    border-color: color-mix(in srgb, var(--color-status-validated) 40%, transparent);
  }
  .status-invited {
    color: var(--color-status-unvalidated);
    border-color: color-mix(in srgb, var(--color-status-unvalidated) 40%, transparent);
  }
  .status-suspended {
    color: var(--color-status-expired);
    border-color: color-mix(in srgb, var(--color-status-expired) 40%, transparent);
  }
  /* Sign-out — muted AĒR red (terracotta), compact, never full-width. */
  .signout {
    flex-shrink: 0;
    align-self: flex-start;
    appearance: none;
    background: transparent;
    border: 1px solid color-mix(in srgb, var(--color-status-expired) 55%, transparent);
    border-radius: var(--radius-md);
    color: var(--color-status-expired);
    font-family: var(--font-ui);
    font-size: var(--font-size-xs);
    font-weight: var(--font-weight-medium);
    padding: var(--space-1) var(--space-3);
    cursor: pointer;
    transition: background var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .signout:hover,
  .signout:focus-visible {
    background: color-mix(in srgb, var(--color-status-expired) 14%, transparent);
    outline: none;
  }
  @media (prefers-reduced-motion: reduce) {
    .signout {
      transition: none;
    }
  }
</style>
