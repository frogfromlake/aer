<script lang="ts">
  import type { Snippet } from 'svelte';

  type Placement = 'top' | 'bottom' | 'left' | 'right';

  interface Props {
    text: string;
    placement?: Placement;
    children?: Snippet;
  }

  let { text, placement = 'top', children }: Props = $props();

  const id = `tooltip-${Math.random().toString(36).slice(2, 10)}`;
  let open = $state(false);

  function show() {
    open = true;
  }
  function hide() {
    open = false;
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<span
  class="anchor"
  aria-describedby={id}
  onmouseenter={show}
  onmouseleave={hide}
  onfocusin={show}
  onfocusout={hide}
>
  {#if children}{@render children()}{/if}
  <span {id} class="tooltip tooltip-{placement}" class:open role="tooltip">{text}</span>
</span>

<style>
  .anchor {
    position: relative;
    display: inline-flex;
    align-items: center;
  }

  .tooltip {
    position: absolute;
    z-index: 100;
    pointer-events: none;
    max-width: 220px;
    padding: var(--space-2) var(--space-3);
    background: var(--color-bg-elevated);
    color: var(--color-fg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    box-shadow: var(--elevation-2);
    font-size: var(--font-size-xs);
    line-height: var(--line-height-base);
    opacity: 0;
    transform: translateY(4px);
    transition:
      opacity var(--motion-duration-fast) var(--motion-ease-standard),
      transform var(--motion-duration-fast) var(--motion-ease-standard);
    white-space: normal;
  }

  .tooltip.open {
    opacity: 1;
    transform: translateY(0);
  }

  .tooltip-top {
    bottom: calc(100% + var(--space-2));
    left: 50%;
    transform: translate(-50%, 4px);
  }
  .tooltip-top.open {
    transform: translate(-50%, 0);
  }

  .tooltip-bottom {
    top: calc(100% + var(--space-2));
    left: 50%;
    transform: translate(-50%, -4px);
  }
  .tooltip-bottom.open {
    transform: translate(-50%, 0);
  }

  .tooltip-left {
    right: calc(100% + var(--space-2));
    top: 50%;
    transform: translate(4px, -50%);
  }
  .tooltip-left.open {
    transform: translate(0, -50%);
  }

  .tooltip-right {
    left: calc(100% + var(--space-2));
    top: 50%;
    transform: translate(-4px, -50%);
  }
  .tooltip-right.open {
    transform: translate(0, -50%);
  }
</style>
