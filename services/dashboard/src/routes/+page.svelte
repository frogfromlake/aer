<svelte:head>
  <title>AĒR — Dashboard</title>
</svelte:head>

<section>
  <h1>Hello AĒR</h1>
  <p>ἀήρ — atmospheric sensor for human discourse.</p>
  <p class="phase">Phase 97 scaffolding. No user-facing features yet.</p>
</section>

<style>
  section {
    text-align: center;
    padding: 2rem;
    max-width: 40rem;
  }

  h1 {
    font-size: clamp(2.5rem, 6vw, 4rem);
    margin: 0 0 1rem;
    letter-spacing: -0.02em;
  }

  p {
    margin: 0.5rem 0;
    line-height: 1.6;
    opacity: 0.85;
  }

  .phase {
    margin-top: 2rem;
    font-size: 0.85rem;
    opacity: 0.5;
  }
</style>
