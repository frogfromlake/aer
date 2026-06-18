<script lang="ts">
  // Your-account as a global overlay (Phase 134 / ADR-040). Same model as the
  // Dossier: a dimmed scrim over the persistent globe + a solid panel. Driven
  // by `?account=open`, so the globe behind never remounts on open/close.
  import * as authApi from '$lib/api/auth';
  import { registerPasskey } from '$lib/api/webauthn-browser';
  import { user, setUser, doLogout, isAdmin } from '$lib/state/auth.svelte';
  import { urlState, setUrl } from '$lib/state/url.svelte';
  import AuthField from '$lib/components/auth/AuthField.svelte';
  import AuthNotice from '$lib/components/auth/AuthNotice.svelte';
  import Button from '$lib/components/base/Button.svelte';
  import LocaleSwitch from '$lib/components/chrome/LocaleSwitch.svelte';
  import AdminPanel from '$lib/components/account/AdminPanel.svelte';
  import { m } from '$lib/paraglide/messages.js';
  import { formatDate } from '$lib/localization/format';

  const MIN_LEN = 12;
  const url = $derived(urlState());
  const me = $derived(user());
  const admin = $derived(isAdmin());

  // Phase 151 — the account overlay is now tabbed (Account · Administration);
  // Administration shows only to admins. Both `?account=open` and `?admin=open`
  // open the overlay, and the URL stays canonical for the active tab so a tab
  // deep-links and round-trips. `?admin=open` for a non-admin falls back to the
  // Account tab.
  const isOpen = $derived(url.account === 'open' || url.admin === 'open');
  const activeTab = $derived<'account' | 'admin'>(
    url.admin === 'open' && admin ? 'admin' : 'account'
  );

  function close() {
    setUrl({ account: null, admin: null });
  }
  function selectTab(tab: 'account' | 'admin') {
    if (tab === 'admin') setUrl({ account: null, admin: 'open' });
    else setUrl({ account: 'open', admin: null });
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
      pwMsg = { kind: 'success', text: m.account_password_changed() };
      currentPw = newPw = confirmPw = '';
    } else {
      pwMsg = {
        kind: 'error',
        text:
          res.code === 'invalid_credentials'
            ? m.account_password_incorrect()
            : m.account_password_change_failed()
      };
    }
  }

  // passkeys
  let passkeys = $state<authApi.PasskeyMeta[]>([]);
  let pkBusy = $state(false);
  let pkMsg = $state<{ kind: 'error' | 'success'; text: string } | null>(null);

  async function loadPasskeys() {
    const res = await authApi.passkeyList();
    // The BFF returns `credentials: null` for an empty list (Go nil slice);
    // coalesce to [] so `.length` / `{#each}` never see null.
    if (res.ok) passkeys = res.data.credentials ?? [];
  }
  async function addPasskey() {
    pkBusy = true;
    pkMsg = null;
    const res = await registerPasskey();
    pkBusy = false;
    if (res.ok) {
      pkMsg = { kind: 'success', text: m.account_passkeys_added_notice() };
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

  // Load passkeys the first time the overlay opens (not on every page load).
  let loaded = false;
  $effect(() => {
    if (isOpen && !loaded) {
      loaded = true;
      void loadPasskeys();
    }
  });
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
      aria-label={m.account_title()}
      tabindex="-1"
    >
      <header class="head">
        <h2>{m.account_title()}</h2>
        <button type="button" class="close" aria-label={m.common_close()} onclick={close}>×</button>
      </header>

      <!-- Phase 151 — tabbed: Account · Administration (admin-only). -->
      <div class="tabs" role="tablist" aria-label={m.account_tabs_label()}>
        <button
          type="button"
          role="tab"
          id="account-tab-account"
          class="tab"
          class:is-active={activeTab === 'account'}
          aria-selected={activeTab === 'account'}
          aria-controls="account-panel-account"
          onclick={() => selectTab('account')}
        >
          {m.account_tab_account()}
        </button>
        {#if admin}
          <button
            type="button"
            role="tab"
            id="account-tab-admin"
            class="tab"
            class:is-active={activeTab === 'admin'}
            aria-selected={activeTab === 'admin'}
            aria-controls="account-panel-admin"
            onclick={() => selectTab('admin')}
          >
            {m.account_tab_admin()}
          </button>
        {/if}
      </div>

      {#if activeTab === 'account'}
        <div
          id="account-panel-account"
          role="tabpanel"
          aria-labelledby="account-tab-account"
          class="tabpanel"
        >
          <section class="block">
            <h3>{m.account_identity_heading()}</h3>
            <dl class="identity">
              <div>
                <dt>{m.account_identity_email()}</dt>
                <dd>{me?.email ?? '—'}</dd>
              </div>
              <div>
                <dt>{m.account_identity_role()}</dt>
                <dd class="cap">{me?.role ?? '—'}</dd>
              </div>
              <div>
                <dt>{m.account_identity_status()}</dt>
                <dd class="cap">{me?.status ?? '—'}</dd>
              </div>
            </dl>
          </section>

          <section class="block">
            <h3>{m.account_password_heading()}</h3>
            <form onsubmit={changePassword} novalidate>
              {#if pwMsg}<AuthNotice variant={pwMsg.kind}>{pwMsg.text}</AuthNotice>{/if}
              <AuthField
                id="cur"
                label={m.account_password_current_label()}
                type="password"
                bind:value={currentPw}
                autocomplete="current-password"
                disabled={pwBusy}
              />
              <AuthField
                id="new"
                label={m.account_password_new_label()}
                type="password"
                bind:value={newPw}
                autocomplete="new-password"
                disabled={pwBusy}
                hint={m.account_password_min_hint({ min: MIN_LEN })}
              />
              <AuthField
                id="conf"
                label={m.account_password_confirm_label()}
                type="password"
                bind:value={confirmPw}
                autocomplete="new-password"
                disabled={pwBusy}
              />
              <div class="actions">
                <Button type="submit" variant="primary" loading={pwBusy} disabled={!pwValid}
                  >{m.account_password_submit()}</Button
                >
              </div>
            </form>
          </section>

          <section class="block">
            <h3>{m.account_passkeys_heading()}</h3>
            <p class="muted">{m.account_passkeys_intro()}</p>
            {#if pkMsg}<AuthNotice variant={pkMsg.kind}>{pkMsg.text}</AuthNotice>{/if}
            {#if passkeys.length === 0}
              <p class="muted">{m.account_passkeys_empty()}</p>
            {:else}
              <ul class="list">
                {#each passkeys as pk (pk.id)}
                  <li>
                    <span
                      >{m.account_passkeys_added_meta({
                        name: pk.name || m.account_passkeys_default_name(),
                        date: formatDate(pk.createdAt)
                      })}</span
                    >
                    <button type="button" class="link-danger" onclick={() => removePasskey(pk.id)}
                      >{m.common_remove()}</button
                    >
                  </li>
                {/each}
              </ul>
            {/if}
            <div class="actions">
              <Button variant="secondary" loading={pkBusy} onclick={addPasskey}
                >{m.account_passkeys_add()}</Button
              >
            </div>
          </section>

          <section class="block">
            <h3>{m.account_privacy_heading()}</h3>
            <p class="muted">
              {m.account_privacy_intro()}
            </p>
            <div class="actions">
              <Button variant="secondary" loading={exportBusy} onclick={exportData}
                >{m.account_privacy_export()}</Button
              >
            </div>
            <div class="danger">
              <h4>{m.account_privacy_delete_heading()}</h4>
              <p class="muted">
                {m.account_privacy_delete_intro({ token: 'DELETE' })}
              </p>
              <div class="delete-row">
                <input
                  class="confirm-input"
                  placeholder={m.account_privacy_delete_placeholder()}
                  bind:value={deleteConfirm}
                  aria-label={m.account_privacy_delete_confirm_label()}
                />
                <Button
                  variant="secondary"
                  loading={deleteBusy}
                  disabled={deleteConfirm !== 'DELETE'}
                  onclick={deleteAccount}>{m.account_privacy_delete_submit()}</Button
                >
              </div>
            </div>
          </section>

          <section class="block">
            <h3>{m.account_language_heading()}</h3>
            <p class="muted">{m.account_language_intro()}</p>
            <LocaleSwitch />
          </section>

          <section class="block actions">
            <Button variant="secondary" onclick={() => doLogout()}>{m.chrome_user_signout()}</Button
            >
          </section>
        </div>
      {:else if activeTab === 'admin'}
        <div
          id="account-panel-admin"
          role="tabpanel"
          aria-labelledby="account-tab-admin"
          class="tabpanel"
        >
          <AdminPanel />
        </div>
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
    /* Phase 151 — wider to accommodate the Administration tab's user table. */
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
  .tabs {
    display: flex;
    gap: var(--space-1);
    border-bottom: 1px solid var(--color-border);
  }
  .tab {
    appearance: none;
    background: transparent;
    border: none;
    border-bottom: 2px solid transparent;
    color: var(--color-fg-muted);
    font-family: var(--font-ui);
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-medium);
    padding: var(--space-2) var(--space-3);
    margin-bottom: -1px;
    cursor: pointer;
    transition: color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .tab:hover,
  .tab:focus-visible {
    color: var(--color-fg);
    outline: none;
  }
  .tab.is-active {
    color: var(--color-accent);
    border-bottom-color: var(--color-accent);
  }
  .tabpanel {
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
  }
  @media (prefers-reduced-motion: reduce) {
    .tab {
      transition: none;
    }
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
</style>
