<script lang="ts">
  import { onMount } from 'svelte';
  import * as authApi from '$lib/api/auth';
  import { registerPasskey } from '$lib/api/webauthn-browser';
  import { user, setUser, doLogout } from '$lib/state/auth.svelte';
  import AuthField from '$lib/components/auth/AuthField.svelte';
  import AuthNotice from '$lib/components/auth/AuthNotice.svelte';
  import Button from '$lib/components/base/Button.svelte';
  import GlobeBackdrop from '$lib/components/atmosphere/GlobeBackdrop.svelte';

  const MIN_LEN = 12;
  const me = $derived(user());

  // change password
  let currentPw = $state('');
  let newPw = $state('');
  let confirmPw = $state('');
  let pwBusy = $state(false);
  let pwMsg = $state<{ kind: 'error' | 'success'; text: string } | null>(null);

  const pwValid = $derived(newPw.length >= MIN_LEN && newPw === confirmPw && currentPw.length > 0);

  async function changePassword(event: SubmitEvent) {
    event.preventDefault();
    if (!pwValid || pwBusy) return;
    pwBusy = true;
    pwMsg = null;
    const res = await authApi.changePassword(currentPw, newPw);
    pwBusy = false;
    if (res.ok) {
      pwMsg = { kind: 'success', text: 'Password changed. Other sessions were signed out.' };
      currentPw = newPw = confirmPw = '';
    } else {
      pwMsg = {
        kind: 'error',
        text:
          res.code === 'invalid_credentials'
            ? 'Current password is incorrect.'
            : 'Could not change the password.'
      };
    }
  }

  // passkeys
  let passkeys = $state<authApi.PasskeyMeta[]>([]);
  let pkBusy = $state(false);
  let pkMsg = $state<{ kind: 'error' | 'success'; text: string } | null>(null);

  async function loadPasskeys() {
    const res = await authApi.passkeyList();
    if (res.ok) passkeys = res.data.credentials;
  }
  async function addPasskey() {
    pkBusy = true;
    pkMsg = null;
    const res = await registerPasskey();
    pkBusy = false;
    if (res.ok) {
      pkMsg = { kind: 'success', text: 'Passkey added.' };
      await loadPasskeys();
    } else {
      pkMsg = { kind: 'error', text: res.message };
    }
  }
  async function removePasskey(id: string) {
    const res = await authApi.passkeyDelete(id);
    if (res.ok) await loadPasskeys();
  }

  // privacy
  let exportBusy = $state(false);
  let deleteConfirm = $state('');
  let deleteBusy = $state(false);

  async function exportData() {
    exportBusy = true;
    const res = await authApi.exportMyData();
    exportBusy = false;
    if (res.ok) {
      const blob = new Blob([JSON.stringify(res.data, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'aer-my-data.json';
      a.click();
      URL.revokeObjectURL(url);
    }
  }
  async function deleteAccount() {
    if (deleteConfirm !== 'DELETE' || deleteBusy) return;
    deleteBusy = true;
    const res = await authApi.deleteMyAccount();
    deleteBusy = false;
    if (res.ok) {
      setUser(null);
      await doLogout();
    }
  }

  onMount(loadPasskeys);
</script>

<svelte:head><title>Your account · AĒR</title></svelte:head>

<GlobeBackdrop />

<main class="settings">
  <header class="page-head">
    <h1>Your account</h1>
  </header>

  <section class="panel">
    <h2>Identity</h2>
    <dl class="identity">
      <div>
        <dt>Email</dt>
        <dd>{me?.email ?? '—'}</dd>
      </div>
      <div>
        <dt>Role</dt>
        <dd class="cap">{me?.role ?? '—'}</dd>
      </div>
      <div>
        <dt>Status</dt>
        <dd class="cap">{me?.status ?? '—'}</dd>
      </div>
    </dl>
  </section>

  <section class="panel">
    <h2>Change password</h2>
    <form onsubmit={changePassword} novalidate>
      {#if pwMsg}<AuthNotice variant={pwMsg.kind}>{pwMsg.text}</AuthNotice>{/if}
      <AuthField
        id="cur"
        label="Current password"
        type="password"
        bind:value={currentPw}
        autocomplete="current-password"
        disabled={pwBusy}
      />
      <AuthField
        id="new"
        label="New password"
        type="password"
        bind:value={newPw}
        autocomplete="new-password"
        disabled={pwBusy}
        hint={`At least ${MIN_LEN} characters.`}
      />
      <AuthField
        id="conf"
        label="Confirm new password"
        type="password"
        bind:value={confirmPw}
        autocomplete="new-password"
        disabled={pwBusy}
      />
      <div class="actions">
        <Button type="submit" variant="primary" loading={pwBusy} disabled={!pwValid}
          >Update password</Button
        >
      </div>
    </form>
  </section>

  <section class="panel">
    <h2>Passkeys</h2>
    <p class="muted">Phishing-resistant sign-in with your device or a security key.</p>
    {#if pkMsg}<AuthNotice variant={pkMsg.kind}>{pkMsg.text}</AuthNotice>{/if}
    {#if passkeys.length === 0}
      <p class="muted">No passkeys registered yet.</p>
    {:else}
      <ul class="list">
        {#each passkeys as pk (pk.id)}
          <li>
            <span>{pk.name || 'Passkey'} · added {new Date(pk.createdAt).toLocaleDateString()}</span
            >
            <button type="button" class="link-danger" onclick={() => removePasskey(pk.id)}
              >Remove</button
            >
          </li>
        {/each}
      </ul>
    {/if}
    <div class="actions">
      <Button variant="secondary" loading={pkBusy} onclick={addPasskey}>Add a passkey</Button>
    </div>
  </section>

  <section class="panel">
    <h2>Data &amp; privacy</h2>
    <p class="muted">
      AĒR stores only your identity and consent — never a record of what you analyse.
    </p>
    <div class="actions">
      <Button variant="secondary" loading={exportBusy} onclick={exportData}>Export my data</Button>
    </div>

    <div class="danger">
      <h3>Delete account</h3>
      <p class="muted">Permanent and irreversible. Type <code>DELETE</code> to confirm.</p>
      <div class="delete-row">
        <input
          class="confirm-input"
          placeholder="DELETE"
          bind:value={deleteConfirm}
          aria-label="Type DELETE to confirm"
        />
        <Button
          variant="secondary"
          loading={deleteBusy}
          disabled={deleteConfirm !== 'DELETE'}
          onclick={deleteAccount}
        >
          Delete my account
        </Button>
      </div>
    </div>
  </section>
</main>

<style>
  .settings {
    position: relative;
    z-index: 1;
    padding: var(--space-6) var(--space-6) var(--space-8);
    padding-left: calc(var(--rail-width) + var(--space-6));
    max-width: calc(var(--rail-width) + 44rem);
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
  form {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .actions {
    display: flex;
    gap: var(--space-3);
    margin-top: var(--space-1);
  }
  .identity {
    display: grid;
    gap: var(--space-3);
    margin: 0;
  }
  .identity div {
    display: flex;
    justify-content: space-between;
    gap: var(--space-4);
  }
  .identity dt {
    color: var(--color-fg-subtle);
    font-size: var(--font-size-sm);
  }
  .identity dd {
    margin: 0;
    color: var(--color-fg);
    font-size: var(--font-size-sm);
  }
  .cap {
    text-transform: capitalize;
  }
  .muted {
    color: var(--color-fg-muted);
    font-size: var(--font-size-sm);
    margin: 0;
    line-height: var(--line-height-base);
  }
  .list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .list li {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--space-3);
    font-size: var(--font-size-sm);
    color: var(--color-fg);
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: var(--space-2) var(--space-3);
  }
  .link-danger {
    background: none;
    border: none;
    color: var(--color-status-expired);
    font-size: var(--font-size-sm);
    cursor: pointer;
  }
  .link-danger:hover {
    text-decoration: underline;
  }
  .danger {
    border-top: 1px solid var(--color-border);
    padding-top: var(--space-4);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .danger h3 {
    margin: 0;
    font-size: var(--font-size-base);
    color: var(--color-status-expired);
    font-weight: var(--font-weight-medium);
  }
  .delete-row {
    display: flex;
    gap: var(--space-3);
    align-items: center;
    margin-top: var(--space-2);
  }
  .confirm-input {
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-fg);
    padding: var(--space-2) var(--space-3);
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
  }
  .confirm-input:focus-visible {
    outline: none;
    border-color: var(--color-accent);
    box-shadow: 0 0 0 var(--focus-ring-width)
      color-mix(in oklab, var(--color-accent) 40%, transparent);
  }
  code {
    font-family: var(--font-mono);
    color: var(--color-fg);
  }
</style>
