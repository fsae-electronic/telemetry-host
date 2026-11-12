<script>
  import StatusBar  from './lib/components/StatusBar.svelte';
  import Dashboard  from './routes/Dashboard.svelte';
  import Runs       from './routes/Runs.svelte';
  import Health     from './routes/Health.svelte';
  import Compare    from './routes/Compare.svelte';
  import { page } from './lib/nav.js';
  import { liveEvents } from './lib/ws.js';

  const TABS = [
    ['dashboard', 'Dashboard'],
    ['health',    'Health'],
    ['runs',      'Runs'],
    ['compare',   'Compare'],
  ];

  // Errors seen live while another tab is open, shown as a badge on Health.
  let seenErrors = 0;
  $: errorCount = $liveEvents.filter(e => e.level === 'error').length;
  $: if ($page === 'health' || errorCount < seenErrors) seenErrors = errorCount;
  $: unseenErrors = errorCount - seenErrors;
</script>

<div class="app">
  <StatusBar />

  <nav>
    {#each TABS as [key, label]}
      <button class:active={$page === key} on:click={() => page.set(key)}>
        {label}
        {#if key === 'health' && unseenErrors > 0}<span class="badge">{unseenErrors}</span>{/if}
      </button>
    {/each}
  </nav>

  <main>
    {#if $page === 'dashboard'}
      <Dashboard />
    {:else if $page === 'health'}
      <Health />
    {:else if $page === 'runs'}
      <Runs />
    {:else if $page === 'compare'}
      <Compare />
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
  .badge {
    margin-left: 4px;
    padding: 0 5px;
    border-radius: 8px;
    background: #c62828;
    color: #fff;
    font-size: 0.65rem;
  }

  main { flex: 1; overflow: hidden; }
</style>
