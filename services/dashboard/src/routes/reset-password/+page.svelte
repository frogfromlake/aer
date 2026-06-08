<script lang="ts">
  /* eslint-disable svelte/no-navigation-without-resolve -- internal auth route */
  import { onMount } from 'svelte';
  import * as authApi from '$lib/api/auth';
  import AuthCard from '$lib/components/auth/AuthCard.svelte';
  import AuthField from '$lib/components/auth/AuthField.svelte';
  import AuthNotice from '$lib/components/auth/AuthNotice.svelte';
  import Button from '$lib/components/base/Button.svelte';

  const MIN_LEN = 12;

  let token = $state('');
  let password = $state('');
  let confirm = $state('');
  let error = $state<string | null>(null);
  let submitting = $state(false);
  let done = $state(false);

  onMount(() => {
    token = new URLSearchParams(window.location.search).get('token') ?? '';
  });

  const mismatch = $derived(confirm.length > 0 && password !== confirm);
  const canSubmit = $derived(
    token.length > 0 && password.length >= MIN_LEN && password === confirm && !submitting
  );

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    if (!canSubmit) return;
    error = null;
    submitting = true;
    const res = await authApi.resetPassword(token, password);
    submitting = false;
    if (res.ok) {
      done = true;
      return;
    }
    error =
      res.code === 'invalid_token'
        ? 'This reset link is invalid or has expired. Request a new one.'
        : res.code === 'weak_password'
          ? `Choose a password of at least ${MIN_LEN} characters.`
          : 'Could not reset the password. Please try again.';
  }
</script>

<svelte:head><title>Set a new password · AĒR</title></svelte:head>

<AuthCard title="Set a new password" subtitle="Choose a new password for your account.">
  {#if done}
    <AuthNotice variant="success">Your password has been reset. You can now sign in.</AuthNotice>
  {:else}
    <form onsubmit={submit} novalidate>
      {#if error}
        <AuthNotice variant="error">{error}</AuthNotice>
      {/if}
      {#if !token}
        <AuthNotice variant="error">This link is missing its reset token.</AuthNotice>
      {/if}

      <AuthField
        id="password"
        label="New password"
        type="password"
        bind:value={password}
        autocomplete="new-password"
        required
        disabled={submitting}
        hint={`At least ${MIN_LEN} characters.`}
      />
      <AuthField
        id="confirm"
        label="Confirm password"
        type="password"
        bind:value={confirm}
        autocomplete="new-password"
        required
        disabled={submitting}
      />
      {#if mismatch}
        <span class="inline-warn">Passwords do not match.</span>
      {/if}

      <Button type="submit" variant="primary" loading={submitting} disabled={!canSubmit}>
        Reset password
      </Button>
    </form>
  {/if}

  {#snippet footer()}
    <a class="link" href="/login">Back to sign in</a>
  {/snippet}
</AuthCard>

<style>
  form {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  form :global(.btn) {
    width: 100%;
    margin-top: var(--space-2);
  }
  .inline-warn {
    font-size: var(--font-size-xs);
    color: var(--color-status-expired);
    margin-top: calc(-1 * var(--space-2));
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
