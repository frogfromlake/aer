<script lang="ts">
  // Administration panel (Phase 151) — the invite + user-management UI, lifted
  // out of the former standalone AdminOverlay so it can render as a TAB inside
  // the account overlay (operator-led design pass). Presentational: no scrim,
  // no dialog role, no Escape handler — the host overlay owns those.
  //
  // Behaviour preserved verbatim from AdminOverlay (Phase 134 / ADR-040):
  // invite a user, list users, suspend/reactivate, reset-password link. Mounts
  // only when the Administration tab is active, so the user list loads lazily.
  import * as authApi from '$lib/api/auth';
  import { isAdmin } from '$lib/state/auth.svelte';
  import AuthField from '$lib/components/auth/AuthField.svelte';
  import AuthNotice from '$lib/components/auth/AuthNotice.svelte';
  import Button from '$lib/components/base/Button.svelte';
  import { statusLabel } from '$lib/account/status-label';
  import { displayName, hasName } from '$lib/identity/initials';
  import { m } from '$lib/paraglide/messages.js';

  const admin = $derived(isAdmin());

  let users = $state<authApi.AdminUser[]>([]);
  let loadError = $state<string | null>(null);

  let inviteEmail = $state('');
  let inviteRole = $state('researcher');
  let inviteBusy = $state(false);
  let inviteMsg = $state<{ kind: 'error' | 'success'; text: string } | null>(null);
  let lastLink = $state<string | null>(null);
  // Phase 153: whether the last action's link was emailed (true), not emailed
  // (false), or unknown (null, pre-response). Drives the delivery note.
  let lastDelivered = $state<boolean | null>(null);

  async function loadUsers() {
    const res = await authApi.adminListUsers();
    if (res.ok) {
      users = res.data.users ?? [];
      loadError = null;
    } else {
      loadError = res.code === 'forbidden_role' ? m.account_admin_access_required() : res.message;
    }
  }

  async function invite(event: SubmitEvent) {
    event.preventDefault();
    if (inviteBusy) return;
    inviteBusy = true;
    inviteMsg = null;
    const res = await authApi.adminCreateUser(inviteEmail.trim(), inviteRole);
    inviteBusy = false;
    if (res.ok) {
      inviteMsg = {
        kind: 'success',
        text: m.account_admin_invite_success({ email: res.data.email })
      };
      lastLink = res.data.link;
      lastDelivered = res.data.delivered ?? null;
      inviteEmail = '';
      await loadUsers();
    } else {
      inviteMsg = {
        kind: 'error',
        text: res.code === 'email_exists' ? m.account_admin_invite_email_exists() : res.message
      };
    }
  }

  async function suspend(id: string) {
    if ((await authApi.adminSuspend(id)).ok) await loadUsers();
  }
  async function reactivate(id: string) {
    if ((await authApi.adminReactivate(id)).ok) await loadUsers();
  }
  async function resetFor(id: string) {
    const res = await authApi.adminResetPassword(id);
    if (res.ok) {
      lastLink = res.data.link;
      lastDelivered = res.data.delivered ?? null;
    }
  }

  // Load the user list on mount (admins only). This component mounts only when
  // the Administration tab is shown, so this is the lazy-load equivalent of the
  // former overlay's open-once effect.
  let loaded = false;
  $effect(() => {
    if (admin && !loaded) {
      loaded = true;
      void loadUsers();
    }
  });
</script>

{#if !admin}
  <AuthNotice variant="error">{m.account_admin_access_required()}</AuthNotice>
{:else}
  <section class="block">
    <h3>{m.account_admin_invite_heading()}</h3>
    <p class="muted">{m.account_admin_invite_intro()}</p>
    <form onsubmit={invite} novalidate>
      {#if inviteMsg}<AuthNotice variant={inviteMsg.kind}>{inviteMsg.text}</AuthNotice>{/if}
      <div class="invite-row">
        <div class="grow">
          <AuthField
            id="invite-email"
            label={m.account_admin_invite_email_label()}
            type="email"
            bind:value={inviteEmail}
            placeholder={m.account_admin_invite_email_placeholder()}
            required
            disabled={inviteBusy}
          />
        </div>
        <div class="role-field">
          <label for="invite-role">{m.account_admin_invite_role_label()}</label>
          <select id="invite-role" bind:value={inviteRole} disabled={inviteBusy}>
            <option value="researcher">{m.account_admin_invite_role_researcher()}</option>
            <option value="admin">{m.account_admin_invite_role_admin()}</option>
          </select>
        </div>
      </div>
      <div class="actions">
        <Button type="submit" variant="primary" loading={inviteBusy}
          >{m.account_admin_invite_submit()}</Button
        >
      </div>
    </form>
    {#if lastDelivered !== null}
      <p class="delivery {lastDelivered ? 'ok' : 'manual'}">
        {lastDelivered ? m.account_admin_link_delivered() : m.account_admin_link_manual()}
      </p>
    {/if}
    {#if lastLink}
      <div class="link-box">
        <span class="link-label">{m.account_admin_invite_link_label()}</span>
        <code class="link-value">{lastLink}</code>
        <button
          type="button"
          class="copy"
          onclick={() => navigator.clipboard?.writeText(lastLink ?? '')}
          >{m.account_admin_invite_copy()}</button
        >
      </div>
    {/if}
  </section>

  <section class="block">
    <h3>{m.account_admin_users_heading()}</h3>
    {#if loadError}<AuthNotice variant="error">{loadError}</AuthNotice>{/if}
    <div class="table" role="table" aria-label={m.account_admin_users_table_label()}>
      <div class="row head-row" role="row">
        <span role="columnheader">{m.account_admin_users_col_email()}</span>
        <span role="columnheader">{m.account_admin_users_col_role()}</span>
        <span role="columnheader">{m.account_admin_users_col_status()}</span>
        <span role="columnheader" class="ta-right">{m.account_admin_users_col_actions()}</span>
      </div>
      {#each users as u (u.id)}
        <div class="row" role="row">
          <span role="cell" class="user-cell">
            <span class="user-name">{displayName(u)}</span>
            {#if hasName(u)}<span class="user-email">{u.email}</span>{/if}
          </span>
          <span role="cell" class="cap">{u.role}</span>
          <span role="cell" class="cap status-{u.status}">{statusLabel(u.status)}</span>
          <span role="cell" class="row-actions">
            {#if u.status === 'suspended'}
              <button type="button" class="mini" onclick={() => reactivate(u.id)}
                >{m.account_admin_users_reactivate()}</button
              >
            {:else}
              <button type="button" class="mini" onclick={() => suspend(u.id)}
                >{m.account_admin_users_suspend()}</button
              >
            {/if}
            <button type="button" class="mini" onclick={() => resetFor(u.id)}
              >{m.account_admin_users_reset_password()}</button
            >
          </span>
        </div>
      {/each}
    </div>
  </section>
{/if}

<style>
  .block {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .block + .block {
    padding-top: var(--space-4);
    border-top: 1px solid var(--color-border);
  }
  .block h3 {
    margin: 0;
    font-size: var(--font-size-md);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
  }
  .muted {
    color: var(--color-fg-muted);
    font-size: var(--font-size-sm);
    margin: 0;
  }
  form {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .invite-row {
    display: flex;
    gap: var(--space-3);
    align-items: flex-end;
  }
  .grow {
    flex: 1;
  }
  .role-field {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .role-field label {
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-medium);
    color: var(--color-fg-muted);
  }
  select {
    appearance: none;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-fg);
    font-family: var(--font-ui);
    font-size: var(--font-size-base);
    padding: var(--space-3) var(--space-4) var(--space-3) var(--space-3);
  }
  select:focus-visible {
    outline: none;
    border-color: var(--color-accent);
    box-shadow: 0 0 0 var(--focus-ring-width)
      color-mix(in oklab, var(--color-accent) 40%, transparent);
  }
  .actions {
    display: flex;
    gap: var(--space-3);
  }
  .link-box {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-3);
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: var(--space-3);
  }
  .link-label {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
  }
  .delivery {
    margin: 0;
    font-size: var(--font-size-sm);
  }
  .delivery.ok {
    color: var(--color-status-validated);
  }
  .delivery.manual {
    color: var(--color-fg-muted);
  }
  .link-value {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-accent);
    word-break: break-all;
    flex: 1;
    min-width: 12rem;
  }
  .copy {
    background: transparent;
    border: 1px solid var(--color-border-strong);
    border-radius: var(--radius-sm);
    color: var(--color-fg-muted);
    font-size: var(--font-size-xs);
    padding: 2px var(--space-2);
    cursor: pointer;
  }
  .copy:hover {
    color: var(--color-fg);
    border-color: var(--color-accent);
  }
  .table {
    display: flex;
    flex-direction: column;
  }
  .row {
    display: grid;
    grid-template-columns: 2.4fr 1fr 1fr 2fr;
    gap: var(--space-3);
    align-items: center;
    padding: var(--space-3) var(--space-2);
    border-bottom: 1px solid var(--color-border);
    font-size: var(--font-size-sm);
    color: var(--color-fg);
  }
  .row.head-row {
    color: var(--color-fg-subtle);
    font-size: var(--font-size-xs);
    text-transform: uppercase;
    letter-spacing: var(--letter-spacing-wide);
  }
  .email {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  /* User identity cell — display name over the email (Phase 148e). */
  .user-cell {
    display: flex;
    flex-direction: column;
    gap: 1px;
    min-width: 0;
  }
  .user-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--color-fg);
  }
  .user-email {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
  }
  .cap {
    text-transform: capitalize;
  }
  .status-suspended {
    color: var(--color-status-expired);
    text-transform: capitalize;
  }
  .status-invited {
    color: var(--color-status-unvalidated);
    text-transform: capitalize;
  }
  .status-active {
    color: var(--color-status-validated);
    text-transform: capitalize;
  }
  .ta-right {
    text-align: right;
  }
  .row-actions {
    display: flex;
    gap: var(--space-2);
    justify-content: flex-end;
  }
  .mini {
    background: transparent;
    border: 1px solid var(--color-border-strong);
    border-radius: var(--radius-sm);
    color: var(--color-fg-muted);
    font-size: var(--font-size-xs);
    padding: 2px var(--space-2);
    cursor: pointer;
    white-space: nowrap;
  }
  .mini:hover,
  .mini:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-accent);
    outline: none;
  }
</style>
