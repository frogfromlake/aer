<script lang="ts">
  // SkipLink — the visually-hidden "skip to main content" a11y anchor; the first
  // keyboard/screen-reader stop on each page.
  import { m } from '$lib/paraglide/messages.js';

  interface Props {
    href?: string;
    label?: string;
  }

  let { href = '#main', label }: Props = $props();
</script>

<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- fragment anchor within the same page -->
<a {href} class="skip-link">{label ?? m.common_skip_to_main()}</a>
