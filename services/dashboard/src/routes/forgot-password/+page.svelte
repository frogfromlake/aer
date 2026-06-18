<script lang="ts">
  /* eslint-disable svelte/no-navigation-without-resolve -- internal auth route */
  import * as authApi from '$lib/api/auth';
  import AuthCard from '$lib/components/auth/AuthCard.svelte';
  import AuthField from '$lib/components/auth/AuthField.svelte';
  import AuthNotice from '$lib/components/auth/AuthNotice.svelte';
  import Button from '$lib/components/base/Button.svelte';
  import { m } from '$lib/paraglide/messages.js';

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

<svelte:head><title>{m.auth_forgot_doc_title()}</title></svelte:head>

<AuthCard title={m.auth_forgot_title()} subtitle={m.auth_forgot_subtitle()}>
  {#if sent}
    <AuthNotice variant="success">{m.auth_forgot_sent()}</AuthNotice>
  {:else}
    <form onsubmit={submit} novalidate>
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
      <Button type="submit" variant="primary" loading={submitting}>{m.auth_forgot_submit()}</Button>
    </form>
  {/if}

  {#snippet footer()}
    <a class="link" href="/login">{m.auth_back_to_signin()}</a>
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
