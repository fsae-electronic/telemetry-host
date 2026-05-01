<script>
  import StatusBar  from './lib/components/StatusBar.svelte';
  import Dashboard  from './routes/Dashboard.svelte';
  import Runs       from './routes/Runs.svelte';

  let page = 'dashboard';
</script>

<div class="app">
  <StatusBar />

  <nav>
    <button class:active={page === 'dashboard'} on:click={() => page = 'dashboard'}>Dashboard</button>
    <button class:active={page === 'runs'}      on:click={() => page = 'runs'}>Runs</button>
  </nav>

  <main>
    {#if page === 'dashboard'}
      <Dashboard />
    {:else if page === 'runs'}
      <Runs />
    {/if}
  </main>
</div>

<style>
  :global(*, *::before, *::after) { box-sizing: border-box; }
  :global(body) {
    margin: 0;
    background: #0f0f1e;
    color: #e0e0e0;
    font-family: system-ui, -apple-system, 'Segoe UI', sans-serif;
  }
  :global(.uplot) { background: transparent !important; }
  :global(.u-wrap) { background: transparent !important; }
  :global(.u-legend) { background: transparent !important; color: #888 !important; font-size: 0.75rem !important; }

  .app { display: flex; flex-direction: column; height: 100vh; overflow: hidden; }

  nav {
    display: flex;
    background: #141428;
    border-bottom: 1px solid #2a2a4a;
    padding: 0 16px;
    flex-shrink: 0;
  }
  nav button {
    background: none;
    border: none;
    color: #555;
    padding: 7px 16px;
    cursor: pointer;
    font-size: 0.82rem;
    font-weight: 600;
    border-bottom: 2px solid transparent;
    transition: color 0.1s;
  }
  nav button:hover { color: #ccc; }
  nav button.active { color: #4FC3F7; border-bottom-color: #4FC3F7; }

  main { flex: 1; overflow: hidden; }
</style>
