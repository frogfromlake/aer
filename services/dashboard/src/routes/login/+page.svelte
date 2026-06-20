<script lang="ts">
  /* eslint-disable svelte/no-navigation-without-resolve -- internal auth routes */
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import * as authApi from '$lib/api/auth';
  import { setUser, refreshMe } from '$lib/state/auth.svelte';
  import { safeRedirect } from '$lib/auth/safe-redirect';
  import AuthCard from '$lib/components/auth/AuthCard.svelte';
  import AuthField from '$lib/components/auth/AuthField.svelte';
  import AuthNotice from '$lib/components/auth/AuthNotice.svelte';
  import Button from '$lib/components/base/Button.svelte';
  import { m } from '$lib/paraglide/messages.js';

  let email = $state('');
  let password = $state('');
  let error = $state<string | null>(null);
  let submitting = $state(false);

  function redirectTarget(): string {
    return safeRedirect(new URLSearchParams(window.location.search).get('redirect'));
  }

  onMount(async () => {
    // Already signed in? Skip straight through. `replaceState` so the auth page
    // never lingers in the history back-stack — otherwise the SideRail
    // back-arrow (Phase 127) would land on /login and bounce straight back here.
    const u = await refreshMe();
    if (u) await goto(redirectTarget(), { replaceState: true });
  });

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    if (submitting) return;
    error = null;
    submitting = true;
    const res = await authApi.login(email.trim(), password);
    submitting = false;
    if (res.ok) {
      setUser(res.data);
      // `replaceState` — drop /login from history so back-navigation can't
      // return to (and flash) the login screen after a successful sign-in.
      await goto(redirectTarget(), { replaceState: true });
      return;
    }
    error = res.status === 429 ? m.auth_login_error_rate_limited() : m.auth_login_error_invalid();
  }
</script>

<svelte:head><title>{m.auth_login_doc_title()}</title></svelte:head>

<AuthCard title={m.auth_login_title()} subtitle={m.auth_login_subtitle()}>
  <form onsubmit={submit} novalidate>
    {#if error}
      <AuthNotice variant="error">{error}</AuthNotice>
    {/if}

    <AuthField
      id="email"
      label={m.auth_field_email_label()}
      type="email"
      bind:value={email}
      autocomplete="username"
      placeholder={m.auth_field_email_placeholder()}
      required
      disabled={submitting}
    />
    <AuthField
      id="password"
      label={m.auth_field_password_label()}
      type="password"
      bind:value={password}
      autocomplete="current-password"
      required
      disabled={submitting}
    />

    <Button type="submit" variant="primary" loading={submitting}>{m.auth_login_submit()}</Button>
  </form>

  {#snippet footer()}
    <a class="link" href="/forgot-password">{m.auth_login_forgot_link()}</a>
  {/snippet}
</AuthCard>

<style>
  form {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  form :global(.btn) {
    width: 100%;
    margin-top: var(--space-1);
  }
  .link {
    color: var(--color-accent);
    text-decoration: none;
  }
  .link:hover,
  .link:focus-visible {
    text-decoration: underline;
  }
</style>
