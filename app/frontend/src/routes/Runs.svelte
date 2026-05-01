<script>
  import { onMount } from 'svelte';
  import { apiGet, apiDelete } from '../lib/ws.js';
  import { CHANNEL_COLORS, channelLabel } from '../lib/colors.js';
  import UPlotChart from '../lib/components/UPlotChart.svelte';

  let runs = [];
  let loading = true;
  let error = '';
  let selectedRun = null;
  let samples = null;
  let samplesLoading = false;

  const NUM_CH = 12;

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

  async function selectRun(run) {
    selectedRun = run;
    samplesLoading = true;
    samples = null;
    try {
      const raw = await apiGet(`/api/runs/${run.id}/samples`);
      samples = buildUplotData(raw);
    } catch (e) {
      error = e.message;
    } finally {
      samplesLoading = false;
    }
  }

  async function deleteRun(run, e) {
    e.stopPropagation();
    if (!confirm(`Delete "${run.name}"?`)) return;
    try {
      await apiDelete(`/api/runs/${run.id}`);
      if (selectedRun?.id === run.id) { selectedRun = null; samples = null; }
      await loadRuns();
    } catch (e) {
      error = e.message;
    }
  }

  function buildUplotData(raw) {
    if (!raw || raw.length === 0) return null;
    const ts = raw.map(r => r.ts);
    const channels = Array.from({ length: NUM_CH }, (_, i) => {
      const key = `s_${String(i).padStart(2,'0')}`;
      return raw.map(r => r[key] ?? null);
    });
    return [ts, ...channels];
  }

  const runSeries = Array.from({ length: NUM_CH }, (_, i) => ({
    label: channelLabel(i),
    stroke: CHANNEL_COLORS[i],
    width: 1.5,
  }));

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

<div class="runs-page">
  <div class="run-list">
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
            <button
              class="run-item"
              class:selected={selectedRun?.id === run.id}
              on:click={() => selectRun(run)}
            >
              <div class="run-name">{run.name}</div>
              <div class="run-meta">{fmtTime(run.started_at)} · {duration(run)}</div>
              <span class="del" on:click|stopPropagation={(e) => deleteRun(run, e)} role="button" tabindex="0" on:keydown={(e) => e.key === 'Enter' && deleteRun(run, e)} title="Delete">✕</span>
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </div>

  <div class="run-detail">
    {#if !selectedRun}
      <div class="empty">Select a run to view its data.</div>
    {:else if samplesLoading}
      <div class="empty">Loading samples…</div>
    {:else if !samples}
      <div class="empty">No samples in this run.</div>
    {:else}
      <h3>{selectedRun.name}</h3>
      <p class="meta">{fmtTime(selectedRun.started_at)} → {fmtTime(selectedRun.ended_at)} · {duration(selectedRun)}</p>
      <div class="chart-wrap">
        <UPlotChart
          data={samples}
          series={runSeries}
          width={820}
          height={380}
          live={false}
        />
      </div>
    {/if}
  </div>
</div>

<style>
  .runs-page {
    display: grid;
    grid-template-columns: 280px 1fr;
    height: calc(100vh - 50px);
    overflow: hidden;
  }
  .run-list {
    border-right: 1px solid #2a2a4a;
    overflow-y: auto;
    padding: 12px;
  }
  .list-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 10px;
  }
  h2 { font-size: 0.9rem; color: #aaa; text-transform: uppercase; margin: 0; }
  ul { list-style: none; padding: 0; margin: 0; }
  .run-item {
    padding: 10px 12px;
    border-radius: 6px;
    cursor: pointer;
    position: relative;
    margin-bottom: 4px;
    border: 1px solid transparent;
  }
  .run-item:hover { background: #1e1e38; }
  .run-item.selected { background: #1e1e38; border-color: #4FC3F7; }
  .run-name { font-size: 0.9rem; color: #e0e0e0; }
  .run-meta { font-size: 0.75rem; color: #666; margin-top: 2px; }
  .del {
    position: absolute;
    right: 8px;
    top: 50%;
    transform: translateY(-50%);
    background: none;
    border: none;
    color: #555;
    cursor: pointer;
    font-size: 0.75rem;
    padding: 2px 4px;
  }
  .del:hover { color: #e57373; }
  .run-detail {
    padding: 20px;
    overflow-y: auto;
  }
  .empty {
    color: #555;
    margin-top: 60px;
    text-align: center;
    font-size: 0.9rem;
  }
  h3 { margin: 0 0 4px; color: #e0e0e0; }
  .meta { font-size: 0.8rem; color: #666; margin: 0 0 16px; }
  .chart-wrap {
    background: #141428;
    border-radius: 8px;
    padding: 12px;
    overflow-x: auto;
  }
  .muted { color: #555; font-size: 0.85rem; }
  .err { color: #e57373; font-size: 0.85rem; }
  .btn-ghost {
    background: none;
    border: 1px solid #3a3a5a;
    color: #aaa;
    padding: 3px 8px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.8rem;
  }
</style>
