<script lang="ts">
  /* eslint-disable svelte/no-navigation-without-resolve -- internal account/admin routes */
  // Top-right profile menu (Phase 134 / ADR-040): identity, account, admin
  // (admins only), and sign-out. Reads the cached current user; renders nothing
  // until a user is known (the (app) layout guards the route).
  import { user, isAdmin, doLogout } from '$lib/state/auth.svelte';

  let open = $state(false);
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
  <div class="profile" bind:this={rootEl}>
    <button
      type="button"
      class="avatar"
      aria-haspopup="menu"
      aria-expanded={open}
      aria-label="Account menu"
      onclick={toggle}
    >
      {initial}
    </button>

    {#if open}
      <div class="menu" role="menu">
        <div class="identity">
          <span class="email" title={current.email}>{current.email}</span>
          <span class="role">{current.role}</span>
        </div>
        <div class="divider"></div>
        <a class="item" role="menuitem" href="/account" onclick={close}>Your account</a>
        {#if isAdmin()}
          <a class="item" role="menuitem" href="/admin" onclick={close}>Administration</a>
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
  .profile {
    position: fixed;
    top: var(--space-2);
    right: var(--space-4);
    z-index: 500;
  }
  .avatar {
    width: 32px;
    height: 32px;
    border-radius: var(--radius-pill);
    border: 1px solid var(--color-border-strong);
    background: var(--color-surface);
    color: var(--color-fg);
    font-family: var(--font-ui);
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-semibold);
    cursor: pointer;
    display: grid;
    place-items: center;
    transition:
      background var(--motion-duration-fast),
      border-color var(--motion-duration-fast);
  }
  .avatar:hover,
  .avatar:focus-visible {
    background: var(--color-surface-hover);
    border-color: var(--color-accent);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }
  .menu {
    position: absolute;
    top: calc(100% + var(--space-2));
    right: 0;
    min-width: 13rem;
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
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: var(--space-2) var(--space-3);
  }
  .email {
    font-size: var(--font-size-sm);
    color: var(--color-fg);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 14rem;
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
