<script>
  import StatusBar  from './lib/components/StatusBar.svelte';
  import Dashboard  from './routes/Dashboard.svelte';
  import Runs       from './routes/Runs.svelte';
  import Alerts     from './routes/Alerts.svelte';
  import { activeAlerts } from './lib/ruleEngine.js';

  let page = 'dashboard';
</script>

<div class="app">
  <StatusBar />

  <nav>
    <button class:active={page === 'dashboard'} on:click={() => page = 'dashboard'}>Dashboard</button>
    <button class:active={page === 'runs'}      on:click={() => page = 'runs'}>Runs</button>
    <button class:active={page === 'alerts'}    on:click={() => page = 'alerts'} class="alerts-tab">
      Alerts
      {#if $activeAlerts.size > 0}
        <span class="alert-badge">{$activeAlerts.size}</span>
      {/if}
    </button>
  </nav>

  <main>
    {#if page === 'dashboard'}
      <Dashboard />
    {:else if page === 'runs'}
      <Runs />
    {:else if page === 'alerts'}
      <Alerts />
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

  .alerts-tab { position: relative; }
  .alert-badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background: #E57373;
    color: #fff;
    font-size: 0.65rem;
    font-weight: 700;
    border-radius: 8px;
    min-width: 16px;
    height: 16px;
    padding: 0 4px;
    margin-left: 5px;
    vertical-align: middle;
    line-height: 1;
  }

  main { flex: 1; overflow: hidden; }
</style>
