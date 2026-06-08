<script lang="ts">
  /* eslint-disable svelte/no-navigation-without-resolve -- internal auth routes */
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import * as authApi from '$lib/api/auth';
  import { setUser } from '$lib/state/auth.svelte';
  import AuthCard from '$lib/components/auth/AuthCard.svelte';
  import AuthField from '$lib/components/auth/AuthField.svelte';
  import AuthNotice from '$lib/components/auth/AuthNotice.svelte';
  import Button from '$lib/components/base/Button.svelte';

  const MIN_LEN = 12;

  let token = $state('');
  let password = $state('');
  let confirm = $state('');
  let consent = $state(false);
  let error = $state<string | null>(null);
  let submitting = $state(false);

  onMount(() => {
    token = new URLSearchParams(window.location.search).get('token') ?? '';
  });

  const tooShort = $derived(password.length > 0 && password.length < MIN_LEN);
  const mismatch = $derived(confirm.length > 0 && password !== confirm);
  const canSubmit = $derived(
    token.length > 0 && password.length >= MIN_LEN && password === confirm && consent && !submitting
  );

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    if (!canSubmit) return;
    error = null;
    submitting = true;
    const res = await authApi.acceptInvite(token, password, consent);
    submitting = false;
    if (res.ok) {
      setUser(res.data);
      await goto('/');
      return;
    }
    error =
      res.code === 'invalid_token'
        ? 'This invitation link is invalid or has expired. Ask an administrator for a new one.'
        : res.code === 'weak_password'
          ? `Choose a password of at least ${MIN_LEN} characters.`
          : res.code === 'consent_required'
            ? 'You must accept the responsible-use agreement to continue.'
            : 'Could not activate the account. Please try again.';
  }
</script>

<svelte:head><title>Accept invitation · AĒR</title></svelte:head>

<AuthCard
  title="Accept your invitation"
  subtitle="Set a password and agree to the terms of use to activate your account."
>
  <form onsubmit={submit} novalidate>
    {#if error}
      <AuthNotice variant="error">{error}</AuthNotice>
    {/if}
    {#if !token}
      <AuthNotice variant="error">This link is missing its invitation token.</AuthNotice>
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
    {#if tooShort}
      <span class="inline-warn">Password must be at least {MIN_LEN} characters.</span>
    {/if}

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

    <label class="consent">
      <input type="checkbox" bind:checked={consent} disabled={submitting} />
      <span>
        I agree to use AĒR for scientific research only, in accordance with the responsible-use
        restrictions of the project licence (§3).
      </span>
    </label>

    <Button type="submit" variant="primary" loading={submitting} disabled={!canSubmit}>
      Activate account
    </Button>
  </form>

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
  .consent {
    display: flex;
    gap: var(--space-3);
    align-items: flex-start;
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    line-height: var(--line-height-base);
    cursor: pointer;
  }
  .consent input {
    margin-top: 3px;
    accent-color: var(--color-accent);
    width: 16px;
    height: 16px;
    flex-shrink: 0;
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
