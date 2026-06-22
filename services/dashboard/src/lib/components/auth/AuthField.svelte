<script lang="ts">
  // A labelled text/password input for the auth forms. Comfortable sizing
  // (unlike the compact mono inputs elsewhere), full design-token styling, and
  // a visible focus ring.
  interface Props {
    id: string;
    label: string;
    type?: 'text' | 'email' | 'password';
    value: string;
    placeholder?: string;
    autocomplete?: AutoFill;
    required?: boolean;
    disabled?: boolean;
    hint?: string;
    /** Tighter type + padding for dense surfaces (e.g. the account overlay),
     *  leaving the comfortable default for the standalone auth pages. */
    compact?: boolean;
  }

  let {
    id,
    label,
    type = 'text',
    value = $bindable(),
    placeholder,
    autocomplete,
    required = false,
    disabled = false,
    hint,
    compact = false
  }: Props = $props();
</script>

<div class="field" class:compact>
  <label for={id}>{label}</label>
  <input
    {id}
    {type}
    {placeholder}
    {autocomplete}
    {required}
    {disabled}
    bind:value
    aria-describedby={hint ? `${id}-hint` : undefined}
  />
  {#if hint}
    <span id={`${id}-hint`} class="hint">{hint}</span>
  {/if}
</div>

<style>
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .field.compact {
    gap: var(--space-1);
  }
  label {
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-medium);
    color: var(--color-fg-muted);
  }
  .field.compact label {
    font-size: var(--font-size-xs);
  }
  .field.compact input {
    font-size: var(--font-size-sm);
    padding: var(--space-2) var(--space-3);
  }
  input {
    appearance: none;
    width: 100%;
    box-sizing: border-box;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-fg);
    font-family: var(--font-ui);
    font-size: var(--font-size-base);
    padding: var(--space-3) var(--space-3);
    transition:
      border-color var(--motion-duration-fast) var(--motion-ease-standard),
      box-shadow var(--motion-duration-fast) var(--motion-ease-standard);
  }
  input::placeholder {
    color: var(--color-fg-subtle);
  }
  input:hover:not(:disabled) {
    border-color: var(--color-border-strong);
  }
  input:focus-visible {
    outline: none;
    border-color: var(--color-accent);
    box-shadow: 0 0 0 var(--focus-ring-width)
      color-mix(in oklab, var(--color-accent) 40%, transparent);
  }
  input:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }
  /* Browsers paint a yellow/blue background on autofilled fields. Override it
     with the dark AĒR input background (the box-shadow trick is the only way to
     repaint an autofilled field). */
  input:-webkit-autofill,
  input:-webkit-autofill:hover,
  input:-webkit-autofill:focus,
  input:-webkit-autofill:active {
    -webkit-box-shadow: 0 0 0 1000px var(--color-bg) inset;
    box-shadow: 0 0 0 1000px var(--color-bg) inset;
    -webkit-text-fill-color: var(--color-fg);
    caret-color: var(--color-fg);
    border-color: var(--color-border);
    transition: background-color 9999s ease-in-out 0s;
  }
  .hint {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
  }
</style>
