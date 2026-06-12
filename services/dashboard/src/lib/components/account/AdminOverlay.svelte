<script lang="ts">
  // Administration as a global overlay (Phase 134 / ADR-040). Dimmed scrim over
  // the persistent globe + solid panel, driven by `?admin=open`.
  import { onMount } from 'svelte';
  import * as authApi from '$lib/api/auth';
  import { isAdmin } from '$lib/state/auth.svelte';
  import { urlState, setUrl } from '$lib/state/url.svelte';
  import AuthField from '$lib/components/auth/AuthField.svelte';
  import AuthNotice from '$lib/components/auth/AuthNotice.svelte';
  import Button from '$lib/components/base/Button.svelte';

  const url = $derived(urlState());
  const isOpen = $derived(url.admin === 'open');
  const admin = $derived(isAdmin());

  function close() {
    setUrl({ admin: null });
  }
  function onKeydown(event: KeyboardEvent) {
    if (isOpen && event.key === 'Escape') close();
  }

  let users = $state<authApi.AdminUser[]>([]);
  let loadError = $state<string | null>(null);

  let inviteEmail = $state('');
  let inviteRole = $state('researcher');
  let inviteBusy = $state(false);
  let inviteMsg = $state<{ kind: 'error' | 'success'; text: string } | null>(null);
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

  // Load the user list the first time the overlay opens (admins only).
  let loaded = false;
  $effect(() => {
    if (isOpen && admin && !loaded) {
      loaded = true;
      void loadUsers();
    }
  });

  onMount(() => {});
</script>

<svelte:window onkeydown={onKeydown} />

{#if isOpen}
  <div
    class="overlay-backdrop"
    role="presentation"
    onclick={(e) => {
      if (e.target === e.currentTarget) close();
    }}
  >
    <!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
    <section
      class="overlay-panel"
      role="dialog"
      aria-modal="true"
      aria-label="Administration"
      tabindex="-1"
    >
      <header class="head">
        <h2>Administration</h2>
        <button type="button" class="close" aria-label="Close" onclick={close}>×</button>
      </header>

      {#if !admin}
        <AuthNotice variant="error">Administrator access required.</AuthNotice>
      {:else}
        <section class="block">
          <h3>Invite a user</h3>
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
              <Button type="submit" variant="primary" loading={inviteBusy}>Create invitation</Button
              >
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

        <section class="block">
          <h3>Users</h3>
          {#if loadError}<AuthNotice variant="error">{loadError}</AuthNotice>{/if}
          <div class="table" role="table" aria-label="Users">
            <div class="row head-row" role="row">
              <span role="columnheader">Email</span>
              <span role="columnheader">Role</span>
              <span role="columnheader">Status</span>
              <span role="columnheader" class="ta-right">Actions</span>
            </div>
            {#each users as u (u.id)}
              <div class="row" role="row">
                <span role="cell" class="email">{u.email}</span>
                <span role="cell" class="cap">{u.role}</span>
                <span role="cell" class="cap status-{u.status}">{u.status}</span>
                <span role="cell" class="row-actions">
                  {#if u.status === 'suspended'}
                    <button type="button" class="mini" onclick={() => reactivate(u.id)}
                      >Reactivate</button
                    >
                  {:else}
                    <button type="button" class="mini" onclick={() => suspend(u.id)}>Suspend</button
                    >
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
    </section>
  </div>
{/if}

<style>
  .overlay-backdrop {
    position: fixed;
    inset: 0 0 0 var(--rail-width, 184px);
    background: color-mix(in srgb, var(--color-bg) 70%, transparent);
    backdrop-filter: blur(3px);
    -webkit-backdrop-filter: blur(3px);
    z-index: 40;
    display: grid;
    place-items: center;
    padding: var(--space-5);
  }
  .overlay-panel {
    width: min(52rem, 94%);
    max-height: 88vh;
    overflow-y: auto;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.4);
    padding: var(--space-5);
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
  }
  .head {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .head h2 {
    margin: 0;
    font-size: var(--font-size-lg);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
  }
  .close {
    background: transparent;
    border: none;
    color: var(--color-fg-muted);
    font-size: var(--font-size-xl);
    line-height: 1;
    cursor: pointer;
    padding: 0 var(--space-2);
  }
  .close:hover,
  .close:focus-visible {
    color: var(--color-fg);
    outline: none;
  }
  .block {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
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
