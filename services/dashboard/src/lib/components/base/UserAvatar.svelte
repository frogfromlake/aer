<script lang="ts">
  // Identity avatar — a round dark AĒR-blue disc with the user's initials
  // (Phase 148e, Item 1). Reused by the SideRail account button, the account
  // overlay header, and the saved-analyses owner column. Initials prefer the
  // given+family name and fall back to the email local-part, so a nameless
  // account still renders. Decorative by default (a visible name/email usually
  // sits beside it); pass `label` to give it a standalone accessible name.
  import { initials, type IdentityLike } from '$lib/identity/initials';

  interface Props {
    firstName?: string | null | undefined;
    lastName?: string | null | undefined;
    email?: string | null | undefined;
    /** Disc diameter in px. Font size scales to ~40% of this. */
    size?: number;
    /** Accessible label. When omitted the disc is `aria-hidden` (decorative). */
    label?: string;
  }

  const { firstName, lastName, email, size = 28, label }: Props = $props();

  const id = $derived<IdentityLike>({ firstName, lastName, email });
  const text = $derived(initials(id));
  const fontSize = $derived(Math.round(size * 0.4));
</script>

<span
  class="avatar"
  style="--avatar-size: {size}px; --avatar-font: {fontSize}px;"
  role={label ? 'img' : undefined}
  aria-label={label}
  aria-hidden={label ? undefined : true}
>
  {text}
</span>

<style>
  .avatar {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    width: var(--avatar-size);
    height: var(--avatar-size);
    border-radius: 50%;
    background: var(--color-avatar-bg);
    color: var(--color-avatar-fg);
    font-family: var(--font-ui);
    font-size: var(--avatar-font);
    font-weight: var(--font-weight-semibold);
    line-height: 1;
    letter-spacing: 0.02em;
    user-select: none;
  }
</style>
