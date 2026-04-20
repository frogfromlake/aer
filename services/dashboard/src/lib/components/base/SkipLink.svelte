<script lang="ts">
  interface Props {
    href?: string;
    label?: string;
  }

  let { href = '#main', label = 'Skip to main content' }: Props = $props();
</script>

<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- fragment anchor within the same page -->
<a {href} class="skip-link">{label}</a>
