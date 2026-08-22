<svelte:head>
  <title>AnimePortable</title>
</svelte:head>

<main>
  <h1>AnimePortable</h1>
  <p>Desktop anime client</p>
</main>

<style>
  :global(body) {
    margin: 0;
    background: #111318;
    color: #f4f5f7;
    font-family: system-ui, sans-serif;
  }

  main {
    display: grid;
    min-height: 100vh;
    place-content: center;
    text-align: center;
  }

  h1,
  p {
    margin: 0;
  }

  p {
    margin-top: 0.75rem;
    color: #aeb4c0;
  }
</style>
