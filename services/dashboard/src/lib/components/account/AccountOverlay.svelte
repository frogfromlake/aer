<script lang="ts">
  // Your-account as a global overlay (Phase 134 / ADR-040). Same model as the
  // Dossier: a dimmed scrim over the persistent globe + a solid panel. Driven
  // by `?account=open`, so the globe behind never remounts on open/close.
  import { onMount } from 'svelte';
  import * as authApi from '$lib/api/auth';
  import { registerPasskey } from '$lib/api/webauthn-browser';
  import { user, setUser, doLogout } from '$lib/state/auth.svelte';
  import { urlState, setUrl } from '$lib/state/url.svelte';
  import AuthField from '$lib/components/auth/AuthField.svelte';
  import AuthNotice from '$lib/components/auth/AuthNotice.svelte';
  import Button from '$lib/components/base/Button.svelte';

  const MIN_LEN = 12;
  const url = $derived(urlState());
  const isOpen = $derived(url.account === 'open');
  const me = $derived(user());

  function close() {
    setUrl({ account: null });
  }
  function onKeydown(event: KeyboardEvent) {
    if (isOpen && event.key === 'Escape') close();
  }

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
    if ((await authApi.passkeyDelete(id)).ok) await loadPasskeys();
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
      const dl = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = dl;
      a.download = 'aer-my-data.json';
      a.click();
      URL.revokeObjectURL(dl);
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
      aria-label="Your account"
      tabindex="-1"
    >
      <header class="head">
        <h2>Your account</h2>
        <button type="button" class="close" aria-label="Close" onclick={close}>×</button>
      </header>

      <section class="block">
        <h3>Identity</h3>
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

      <section class="block">
        <h3>Change password</h3>
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

      <section class="block">
        <h3>Passkeys</h3>
        <p class="muted">Phishing-resistant sign-in with your device or a security key.</p>
        {#if pkMsg}<AuthNotice variant={pkMsg.kind}>{pkMsg.text}</AuthNotice>{/if}
        {#if passkeys.length === 0}
          <p class="muted">No passkeys registered yet.</p>
        {:else}
          <ul class="list">
            {#each passkeys as pk (pk.id)}
              <li>
                <span
                  >{pk.name || 'Passkey'} · added {new Date(
                    pk.createdAt
                  ).toLocaleDateString()}</span
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

      <section class="block">
        <h3>Data &amp; privacy</h3>
        <p class="muted">
          AĒR stores only your identity and consent — never a record of what you analyse.
        </p>
        <div class="actions">
          <Button variant="secondary" loading={exportBusy} onclick={exportData}
            >Export my data</Button
          >
        </div>
        <div class="danger">
          <h4>Delete account</h4>
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
              onclick={deleteAccount}>Delete my account</Button
            >
          </div>
        </div>
      </section>
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
    width: min(40rem, 92%);
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
  form {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .actions {
    display: flex;
    gap: var(--space-3);
  }
  .identity {
    display: grid;
    gap: var(--space-2);
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
    padding-top: var(--space-3);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .danger h4 {
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
    flex-wrap: wrap;
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
