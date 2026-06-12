<script lang="ts">
  // SideRail account control (Phase 134 / ADR-040). One button — the user's
  // initial avatar + email — that opens the account submenu (role, Your
  // account, Administration, Sign out). Replaces the former top-right profile
  // menu; the popup opens upward since the button sits at the foot of the rail.
  import { user, isAdmin, doLogout } from '$lib/state/auth.svelte';
  import { setUrl } from '$lib/state/url.svelte';

  let open = $state(false);

  function openAccount() {
    setUrl({ account: 'open' });
    close();
  }
  function openAdmin() {
    setUrl({ admin: 'open' });
    close();
  }
  let rootEl: HTMLElement | undefined = $state();

  const current = $derived(user());
  const initial = $derived((current?.email ?? '?').trim().charAt(0).toUpperCase());

  function toggle() {
    open = !open;
  }
  function close() {
    open = false;
  }
  function onKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') close();
  }
  function onWindowClick(event: MouseEvent) {
    if (open && rootEl && !rootEl.contains(event.target as Node)) close();
  }
</script>

<svelte:window onclick={onWindowClick} onkeydown={onKeydown} />

{#if current}
  <div class="account" bind:this={rootEl}>
    <button
      type="button"
      class="account-btn"
      class:open
      aria-haspopup="menu"
      aria-expanded={open}
      aria-label="Account menu"
      onclick={toggle}
    >
      <span class="avatar" aria-hidden="true">{initial}</span>
      <span class="email" title={current.email}>{current.email}</span>
    </button>

    {#if open}
      <div class="menu" role="menu">
        <div class="identity">
          <span class="role">{current.role}</span>
        </div>
        <button type="button" class="item" role="menuitem" onclick={openAccount}
          >Your account</button
        >
        {#if isAdmin()}
          <button type="button" class="item" role="menuitem" onclick={openAdmin}
            >Administration</button
          >
        {/if}
        <div class="divider"></div>
        <button type="button" class="item signout" role="menuitem" onclick={() => doLogout()}>
          Sign out
        </button>
      </div>
    {/if}
  </div>
{/if}

<style>
  .account {
    position: relative;
  }
  .account-btn {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    width: 100%;
    background: transparent;
    border: none;
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-md);
    cursor: pointer;
    color: var(--color-fg-muted);
    transition:
      background var(--motion-duration-fast),
      color var(--motion-duration-fast);
  }
  .account-btn:hover,
  .account-btn:focus-visible,
  .account-btn.open {
    background: var(--color-surface-hover);
    color: var(--color-fg);
    outline: none;
  }
  .avatar {
    width: 28px;
    height: 28px;
    flex-shrink: 0;
    border-radius: var(--radius-pill);
    border: 1px solid var(--color-border-strong);
    background: var(--color-surface);
    color: var(--color-fg);
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-semibold);
    display: grid;
    place-items: center;
  }
  .email {
    font-size: 12px;
    font-family: var(--font-ui);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .menu {
    position: absolute;
    bottom: calc(100% + var(--space-2));
    left: 0;
    min-width: 13rem;
    z-index: 500;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    box-shadow: var(--elevation-3);
    padding: var(--space-2);
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .identity {
    padding: var(--space-1) var(--space-3) var(--space-2);
  }
  .role {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    text-transform: capitalize;
  }
  .divider {
    height: 1px;
    background: var(--color-border);
    margin: var(--space-1) 0;
  }
  .item {
    display: block;
    width: 100%;
    text-align: left;
    background: transparent;
    border: none;
    color: var(--color-fg-muted);
    font-family: var(--font-ui);
    font-size: var(--font-size-sm);
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-sm);
    text-decoration: none;
    cursor: pointer;
  }
  .item:hover,
  .item:focus-visible {
    background: var(--color-surface-hover);
    color: var(--color-fg);
    outline: none;
  }
  .signout {
    color: var(--color-status-expired);
  }
</style>
