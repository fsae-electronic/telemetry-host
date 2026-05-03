<script>
  import { onMount } from 'svelte';
  import { apiGet, apiDelete } from '../lib/ws.js';
  import RunDashboard from './RunDashboard.svelte';

  let runs = [];
  let loading = true;
  let error = '';
  let selectedRun = null;

  async function loadRuns() {
    loading = true; error = '';
    try {
      runs = await apiGet('/api/runs');
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function deleteRun(run, e) {
    e.stopPropagation();
    if (!confirm(`Delete "${run.name}"?`)) return;
    try {
      await apiDelete(`/api/runs/${run.id}`);
      if (selectedRun?.id === run.id) selectedRun = null;
      await loadRuns();
    } catch (e) {
      error = e.message;
    }
  }

  function fmtTime(ts) {
    if (!ts) return '—';
    return new Date(ts * 1000).toLocaleString();
  }

  function duration(run) {
    if (!run.ended_at) return 'ongoing';
    const s = Math.round(run.ended_at - run.started_at);
    return `${Math.floor(s / 60)}m ${s % 60}s`;
  }

  onMount(loadRuns);
</script>

{#if selectedRun}
  <!-- Full-screen run dashboard; back button returns to the list -->
  <RunDashboard run={selectedRun} onBack={() => selectedRun = null} />
{:else}
  <!-- Run list ────────────────────────────────────────────────────────────── -->
  <div class="runs-page">
    <div class="list-header">
      <h2>Saved Runs</h2>
      <button class="btn-ghost" on:click={loadRuns}>↻ Refresh</button>
    </div>

    {#if loading}
      <p class="muted">Loading…</p>
    {:else if error}
      <p class="err">{error}</p>
    {:else if runs.length === 0}
      <p class="muted">No runs recorded yet.</p>
    {:else}
      <ul>
        {#each runs as run}
          <li>
            <button class="run-item" on:click={() => selectedRun = run}>
              <div class="run-name">{run.name}</div>
              <div class="run-meta">{fmtTime(run.started_at)} · {duration(run)}</div>
              <span
                class="del"
                role="button"
                tabindex="0"
                on:click|stopPropagation={(e) => deleteRun(run, e)}
                on:keydown={(e) => e.key === 'Enter' && deleteRun(run, e)}
                title="Delete"
              >✕</span>
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
{/if}

<style>
  .runs-page {
    padding: 16px;
    max-width: 640px;
  }
  .list-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
  }
  h2 { font-size: 0.8rem; color: #666; text-transform: uppercase; margin: 0; letter-spacing: 0.06em; }

  ul { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 4px; }

  .run-item {
    width: 100%;
    text-align: left;
    padding: 10px 12px;
    border-radius: 6px;
    cursor: pointer;
    position: relative;
    border: 1px solid #1e1e38;
    background: #12122a;
    color: inherit;
    transition: border-color 0.1s;
  }
  .run-item:hover { border-color: #4FC3F7; background: #1a1a38; }
  .run-name { font-size: 0.88rem; color: #e0e0e0; }
  .run-meta { font-size: 0.72rem; color: #555; margin-top: 2px; }
  .del {
    position: absolute;
    right: 8px; top: 50%;
    transform: translateY(-50%);
    color: #333;
    cursor: pointer;
    font-size: 0.7rem;
    padding: 4px 6px;
    border-radius: 3px;
  }
  .del:hover { color: #e57373; background: #1e1e38; }

  .muted { color: #555; font-size: 0.82rem; }
  .err   { color: #e57373; font-size: 0.82rem; }
  .btn-ghost {
    background: none;
    border: 1px solid #2a2a4a;
    color: #666;
    padding: 3px 8px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.8rem;
  }
  .btn-ghost:hover { border-color: #4a4a6a; color: #aaa; }
</style>
