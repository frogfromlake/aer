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
    hint
  }: Props = $props();
</script>

<div class="field">
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
  label {
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-medium);
    color: var(--color-fg-muted);
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
  .hint {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
  }
</style>
