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
  import { m } from '$lib/paraglide/messages.js';

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
        ? m.auth_invite_error_invalid_token()
        : res.code === 'weak_password'
          ? m.auth_password_too_weak({ min: MIN_LEN })
          : res.code === 'consent_required'
            ? m.auth_invite_error_consent_required()
            : m.auth_invite_error_generic();
  }
</script>

<svelte:head><title>{m.auth_invite_doc_title()}</title></svelte:head>

<AuthCard title={m.auth_invite_title()} subtitle={m.auth_invite_subtitle()}>
  <form onsubmit={submit} novalidate>
    {#if error}
      <AuthNotice variant="error">{error}</AuthNotice>
    {/if}
    {#if !token}
      <AuthNotice variant="error">{m.auth_invite_missing_token()}</AuthNotice>
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
    {#if tooShort}
      <span class="inline-warn">{m.auth_password_too_short({ min: MIN_LEN })}</span>
    {/if}

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

    <label class="consent">
      <input type="checkbox" bind:checked={consent} disabled={submitting} />
      <span>{m.auth_invite_consent()}</span>
    </label>

    <Button type="submit" variant="primary" loading={submitting} disabled={!canSubmit}>
      {m.auth_invite_submit()}
    </Button>
  </form>

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
