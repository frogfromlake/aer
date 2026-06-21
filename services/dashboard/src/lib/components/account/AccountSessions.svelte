<script lang="ts">
  // Active-sessions view + "log out everywhere" (SEC-005). Self-contained child
  // of AccountOverlay: loads the user's own active sessions on mount and offers
  // a single log-out-all-devices action for a lost device. Privacy-minimal — the
  // BFF never exposes a session id; `current` marks this device.
  import * as authApi from '$lib/api/auth';
  import { setUser, doLogout } from '$lib/state/auth.svelte';
  import Button from '$lib/components/base/Button.svelte';
  import { m } from '$lib/paraglide/messages.js';
  import { formatDate } from '$lib/localization/format';

  let sessions = $state<authApi.SessionSummary[]>([]);
  let revokeBusy = $state(false);

  async function loadSessions() {
    const res = await authApi.listSessions();
    // The BFF returns a nil slice for an empty list; coalesce to [].
    if (res.ok) sessions = res.data.sessions ?? [];
  }
  async function logoutEverywhere() {
    if (revokeBusy) return;
    revokeBusy = true;
    const res = await authApi.revokeAllSessions();
    revokeBusy = false;
    // The current session is revoked too, so clear local state and bounce to
    // login (doLogout is idempotent — /auth/logout returns 204 either way).
    if (res.ok) {
      setUser(null);
      await doLogout();
    }
  }

  $effect(() => {
    void loadSessions();
  });
</script>

<section class="block">
  <h3>{m.account_sessions_heading()}</h3>
  <p class="muted">{m.account_sessions_intro()}</p>
  {#if sessions.length === 0}
    <p class="muted">{m.account_sessions_empty()}</p>
  {:else}
    <ul class="list">
      {#each sessions as sess, i (i)}
        <li>
          <span
            >{m.account_sessions_item({
              agent: sess.userAgent || m.account_sessions_unknown_device(),
              lastSeen: formatDate(sess.lastSeenAt)
            })}</span
          >
          {#if sess.current}
            <span class="current-badge">{m.account_sessions_current()}</span>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
  <div class="actions">
    <Button variant="secondary" loading={revokeBusy} onclick={logoutEverywhere}
      >{m.account_sessions_revoke_all()}</Button
    >
  </div>
</section>

<style>
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
  .current-badge {
    flex-shrink: 0;
    font-size: var(--font-size-xs);
    font-weight: var(--font-weight-medium);
    color: var(--color-accent);
    border: 1px solid var(--color-accent);
    border-radius: var(--radius-sm);
    padding: 0 var(--space-2);
  }
  .actions {
    display: flex;
    gap: var(--space-3);
  }
</style>
