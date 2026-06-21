<script lang="ts">
  /* eslint-disable svelte/no-navigation-without-resolve -- internal auth route */
  import { onMount } from 'svelte';
  import * as authApi from '$lib/api/auth';
  import AuthCard from '$lib/components/auth/AuthCard.svelte';
  import AuthField from '$lib/components/auth/AuthField.svelte';
  import AuthNotice from '$lib/components/auth/AuthNotice.svelte';
  import Button from '$lib/components/base/Button.svelte';
  import { tokenFromHash } from '$lib/auth/token-from-hash';
  import { m } from '$lib/paraglide/messages.js';

  const MIN_LEN = 12;

  let token = $state('');
  let password = $state('');
  let confirm = $state('');
  let error = $state<string | null>(null);
  let submitting = $state(false);
  let done = $state(false);

  onMount(() => {
    // SEC-009 — the token rides in the URL fragment, not the query string.
    token = tokenFromHash(window.location.hash);
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
        ? m.auth_reset_error_invalid_token()
        : res.code === 'weak_password'
          ? m.auth_password_too_weak({ min: MIN_LEN })
          : m.auth_reset_error_generic();
  }
</script>

<svelte:head><title>{m.auth_reset_doc_title()}</title></svelte:head>

<AuthCard title={m.auth_reset_title()} subtitle={m.auth_reset_subtitle()}>
  {#if done}
    <AuthNotice variant="success">{m.auth_reset_done()}</AuthNotice>
  {:else}
    <form onsubmit={submit} novalidate>
      {#if error}
        <AuthNotice variant="error">{error}</AuthNotice>
      {/if}
      {#if !token}
        <AuthNotice variant="error">{m.auth_reset_missing_token()}</AuthNotice>
      {/if}

      <AuthField
        id="password"
        label={m.auth_field_new_password_label()}
        type="password"
        bind:value={password}
        autocomplete="new-password"
        required
        disabled={submitting}
        hint={m.auth_password_min_hint({ min: MIN_LEN })}
      />
      <AuthField
        id="confirm"
        label={m.auth_field_confirm_password_label()}
        type="password"
        bind:value={confirm}
        autocomplete="new-password"
        required
        disabled={submitting}
      />
      {#if mismatch}
        <span class="inline-warn">{m.auth_password_mismatch()}</span>
      {/if}

      <Button type="submit" variant="primary" loading={submitting} disabled={!canSubmit}>
        {m.auth_reset_submit()}
      </Button>
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
