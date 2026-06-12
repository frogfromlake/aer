<script lang="ts">
  import { onMount } from 'svelte';
  import * as authApi from '$lib/api/auth';
  import { isAdmin } from '$lib/state/auth.svelte';
  import AuthField from '$lib/components/auth/AuthField.svelte';
  import AuthNotice from '$lib/components/auth/AuthNotice.svelte';
  import Button from '$lib/components/base/Button.svelte';
  import GlobeBackdrop from '$lib/components/atmosphere/GlobeBackdrop.svelte';

  const admin = $derived(isAdmin());

  let users = $state<authApi.AdminUser[]>([]);
  let loadError = $state<string | null>(null);

  // invite
  let inviteEmail = $state('');
  let inviteRole = $state('researcher');
  let inviteBusy = $state(false);
  let inviteMsg = $state<{ kind: 'error' | 'success'; text: string } | null>(null);
  // The most recent one-time link produced by an admin action (shown so the
  // operator can deliver it while SMTP delivery is not yet wired).
  let lastLink = $state<string | null>(null);

  async function loadUsers() {
    const res = await authApi.adminListUsers();
    if (res.ok) {
      users = res.data.users;
      loadError = null;
    } else {
      loadError = res.code === 'forbidden_role' ? 'Administrator access required.' : res.message;
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
      inviteMsg = { kind: 'success', text: `Invited ${res.data.email}. Share the link below.` };
      lastLink = res.data.link;
      inviteEmail = '';
      await loadUsers();
    } else {
      inviteMsg = {
        kind: 'error',
        text: res.code === 'email_exists' ? 'A user with this email already exists.' : res.message
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
    if (res.ok) lastLink = res.data.link;
  }

  onMount(() => {
    if (admin) void loadUsers();
  });
</script>

<svelte:head><title>Administration · AĒR</title></svelte:head>

<GlobeBackdrop />

<main class="settings">
  <header class="page-head"><h1>Administration</h1></header>

  {#if !admin}
    <section class="panel">
      <AuthNotice variant="error">Administrator access required.</AuthNotice>
    </section>
  {:else}
    <section class="panel">
      <h2>Invite a user</h2>
      <p class="muted">
        Self-registration is closed; accounts are created by invitation (licence §3.2).
      </p>
      <form onsubmit={invite} novalidate>
        {#if inviteMsg}<AuthNotice variant={inviteMsg.kind}>{inviteMsg.text}</AuthNotice>{/if}
        <div class="invite-row">
          <div class="grow">
            <AuthField
              id="invite-email"
              label="Email"
              type="email"
              bind:value={inviteEmail}
              placeholder="new@institution.org"
              required
              disabled={inviteBusy}
            />
          </div>
          <div class="role-field">
            <label for="invite-role">Role</label>
            <select id="invite-role" bind:value={inviteRole} disabled={inviteBusy}>
              <option value="researcher">researcher</option>
              <option value="admin">admin</option>
            </select>
          </div>
        </div>
        <div class="actions">
          <Button type="submit" variant="primary" loading={inviteBusy}>Create invitation</Button>
        </div>
      </form>

      {#if lastLink}
        <div class="link-box">
          <span class="link-label">One-time link (deliver to the user):</span>
          <code class="link-value">{lastLink}</code>
          <button
            type="button"
            class="copy"
            onclick={() => navigator.clipboard?.writeText(lastLink ?? '')}>Copy</button
          >
        </div>
      {/if}
    </section>

    <section class="panel">
      <h2>Users</h2>
      {#if loadError}<AuthNotice variant="error">{loadError}</AuthNotice>{/if}
      <div class="table" role="table" aria-label="Users">
        <div class="row head" role="row">
          <span role="columnheader">Email</span>
          <span role="columnheader">Role</span>
          <span role="columnheader">Status</span>
          <span role="columnheader" class="ta-right">Actions</span>
        </div>
        {#each users as u (u.id)}
          <div class="row" role="row">
            <span role="cell" class="email">{u.email}</span>
            <span role="cell" class="cap">{u.role}</span>
            <span role="cell" class="cap status status-{u.status}">{u.status}</span>
            <span role="cell" class="row-actions">
              {#if u.status === 'suspended'}
                <button type="button" class="mini" onclick={() => reactivate(u.id)}
                  >Reactivate</button
                >
              {:else}
                <button type="button" class="mini" onclick={() => suspend(u.id)}>Suspend</button>
              {/if}
              <button type="button" class="mini" onclick={() => resetFor(u.id)}
                >Reset password</button
              >
            </span>
          </div>
        {/each}
      </div>
    </section>
  {/if}
</main>

<style>
  .settings {
    position: relative;
    z-index: 1;
    padding: var(--space-6) var(--space-6) var(--space-8);
    padding-left: calc(var(--rail-width) + var(--space-6));
    max-width: calc(var(--rail-width) + 56rem);
    margin: 0 auto;
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
  }
  .page-head h1 {
    font-size: var(--font-size-xl);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    margin: 0;
  }
  .panel {
    background: color-mix(in oklab, var(--color-surface) 80%, transparent);
    backdrop-filter: blur(14px);
    -webkit-backdrop-filter: blur(14px);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    padding: var(--space-5);
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    box-shadow: var(--elevation-2);
  }
  .panel h2 {
    font-size: var(--font-size-md);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    margin: 0;
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
  .row.head {
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
  .cap {
    text-transform: capitalize;
  }
  .status-suspended {
    color: var(--color-status-expired);
  }
  .status-invited {
    color: var(--color-status-unvalidated);
  }
  .status-active {
    color: var(--color-status-validated);
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
