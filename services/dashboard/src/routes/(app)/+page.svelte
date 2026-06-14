<script lang="ts">
  // Atmosphere route (Surface I). The globe itself is rendered PERSISTENTLY by
  // the (app) layout (`AtmosphereSurface`) so it survives navigation to the
  // Workbench / Reflection and never remounts — it only reloads on a full page
  // refresh. This page is intentionally empty: the surface's interactive chrome
  // (selection banner, tooltips, probe nav) is shown by AtmosphereSurface when
  // the active route is `/`.
</script>

<svelte:head>
  <title>AĒR — Atmosphere</title>
</svelte:head>
