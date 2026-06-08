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

  let email = $state('');
  let password = $state('');
  let error = $state<string | null>(null);
  let submitting = $state(false);

  function redirectTarget(): string {
    return safeRedirect(new URLSearchParams(window.location.search).get('redirect'));
  }

  onMount(async () => {
    // Already signed in? Skip straight through.
    const u = await refreshMe();
    if (u) await goto(redirectTarget());
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
      await goto(redirectTarget());
      return;
    }
    error =
      res.status === 429
        ? 'Too many attempts. Please wait a moment and try again.'
        : 'Invalid email or password.';
  }
</script>

<svelte:head><title>Sign in · AĒR</title></svelte:head>

<AuthCard title="Sign in" subtitle="Access is by invitation only.">
  <form onsubmit={submit} novalidate>
    {#if error}
      <AuthNotice variant="error">{error}</AuthNotice>
    {/if}

    <AuthField
      id="email"
      label="Email"
      type="email"
      bind:value={email}
      autocomplete="username"
      placeholder="you@institution.org"
      required
      disabled={submitting}
    />
    <AuthField
      id="password"
      label="Password"
      type="password"
      bind:value={password}
      autocomplete="current-password"
      required
      disabled={submitting}
    />

    <Button type="submit" variant="primary" loading={submitting}>Sign in</Button>
  </form>

  {#snippet footer()}
    <a class="link" href="/forgot-password">Forgot your password?</a>
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
