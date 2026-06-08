<script lang="ts">
  /* eslint-disable svelte/no-navigation-without-resolve -- internal auth route */
  import * as authApi from '$lib/api/auth';
  import AuthCard from '$lib/components/auth/AuthCard.svelte';
  import AuthField from '$lib/components/auth/AuthField.svelte';
  import AuthNotice from '$lib/components/auth/AuthNotice.svelte';
  import Button from '$lib/components/base/Button.svelte';

  let email = $state('');
  let submitting = $state(false);
  let sent = $state(false);

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    if (submitting) return;
    submitting = true;
    // The BFF always returns 202 (no account enumeration); the UI mirrors that.
    await authApi.forgotPassword(email.trim());
    submitting = false;
    sent = true;
  }
</script>

<svelte:head><title>Reset password · AĒR</title></svelte:head>

<AuthCard
  title="Reset your password"
  subtitle="Enter your email and we'll send a reset link if an account exists."
>
  {#if sent}
    <AuthNotice variant="success">
      If an account exists for that address, a password-reset link has been sent.
    </AuthNotice>
  {:else}
    <form onsubmit={submit} novalidate>
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
      <Button type="submit" variant="primary" loading={submitting}>Send reset link</Button>
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
    gap: var(--space-4);
  }
  form :global(.btn) {
    width: 100%;
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
