<script>
  import { onMount } from 'svelte';
  import { apiGet, apiDelete } from '../lib/ws.js';
  import { META_FIELDS, runDuration } from '../lib/runs.js';
  import { page, compareSelection } from '../lib/nav.js';
  import RunDashboard from './RunDashboard.svelte';

  const MAX_COMPARE = 4;

  let runs = [];
  let loading = true;
  let error = '';
  let selectedRun = null;

  let search = '';
  let driverFilter = '';
  let modeFilter = '';
  let checked = new Set($compareSelection);

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
      checked.delete(run.id);
      checked = new Set(checked);
      await loadRuns();
    } catch (e) {
      error = e.message;
    }
  }

  function fmtTime(ts) {
    if (!ts) return '—';
    return new Date(ts * 1000).toLocaleString();
  }

  // Values already used for each metadata field, for filters and autocomplete.
  $: suggestions = Object.fromEntries(
    META_FIELDS.map(f => [f.key, [...new Set(runs.map(r => r[f.key]).filter(Boolean))].sort()])
  );

  $: q = search.trim().toLowerCase();
  $: shown = runs.filter(r =>
    (!driverFilter || r.driver === driverFilter) &&
    (!modeFilter || r.mode === modeFilter) &&
    (!q || ['name', 'notes', ...META_FIELDS.map(f => f.key)].some(k => (r[k] ?? '').toLowerCase().includes(q)))
  );

  function toggleCheck(run) {
    if (checked.has(run.id)) checked.delete(run.id);
    else if (checked.size < MAX_COMPARE) checked.add(run.id);
    checked = new Set(checked);
  }

  function openCompare() {
    compareSelection.set([...checked]);
    page.set('compare');
  }

  function onUpdated(updated) {
    runs = runs.map(r => r.id === updated.id ? { ...r, ...updated } : r);
    if (selectedRun?.id === updated.id) selectedRun = { ...selectedRun, ...updated };
  }

  onMount(loadRuns);
</script>

{#if selectedRun}
  <!-- Full-screen run dashboard; back button returns to the list -->
  <RunDashboard
    run={selectedRun}
    {suggestions}
    {onUpdated}
    onBack={() => selectedRun = null}
  />
{:else}
  <!-- Run list ────────────────────────────────────────────────────────────── -->
  <div class="runs-page">
    <div class="list-header">
      <h2>Saved Runs</h2>
      <div class="header-actions">
        <button class="btn-compare" disabled={checked.size < 2} on:click={openCompare}
          title="Tick 2 to {MAX_COMPARE} runs to compare them">
          Compare ({checked.size})
        </button>
        <button class="btn-ghost" on:click={loadRuns}>↻ Refresh</button>
      </div>
    </div>

    <div class="filters">
      <input type="search" placeholder="Search name, driver, track, notes…" bind:value={search} />
      <select bind:value={driverFilter}>
        <option value="">All drivers</option>
        {#each suggestions.driver ?? [] as d}<option value={d}>{d}</option>{/each}
      </select>
      <select bind:value={modeFilter}>
        <option value="">All modes</option>
        {#each suggestions.mode ?? [] as m}<option value={m}>{m}</option>{/each}
      </select>
    </div>

    {#if loading}
      <p class="muted">Loading…</p>
    {:else if error}
      <p class="err">{error}</p>
    {:else if runs.length === 0}
      <p class="muted">No runs recorded yet.</p>
    {:else if shown.length === 0}
      <p class="muted">No runs match the filters.</p>
    {:else}
      <ul>
        {#each shown as run (run.id)}
          <li class="run-row">
            <input
              type="checkbox"
              class="check"
              checked={checked.has(run.id)}
              disabled={!checked.has(run.id) && checked.size >= MAX_COMPARE}
              on:change={() => toggleCheck(run)}
              title="Select for comparison"
            />
            <button class="run-item" on:click={() => selectedRun = run}>
              <div class="run-name">{run.name}</div>
              <div class="run-meta">
                {fmtTime(run.started_at)} · {runDuration(run)}
                {#if run.markers}<span class="flag">⚑ {run.markers}</span>{/if}
              </div>
              {#if META_FIELDS.some(f => run[f.key]) || run.notes}
                <div class="run-tags">
                  {#each META_FIELDS as f}
                    {#if run[f.key]}<span class="tag" title={f.label}>{run[f.key]}</span>{/if}
                  {/each}
                  {#if run.notes}<span class="notes">{run.notes}</span>{/if}
                </div>
              {/if}
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
    max-width: 820px;
    height: 100%;
    overflow-y: auto;
  }
  .list-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
  }
  .header-actions { display: flex; gap: 6px; }
  h2 { font-size: 0.8rem; color: #666; text-transform: uppercase; margin: 0; letter-spacing: 0.06em; }

  .filters { display: flex; gap: 6px; margin-bottom: 10px; }
  .filters input, .filters select {
    background: #12122a;
    border: 1px solid #2a2a4a;
    color: #e0e0e0;
    padding: 5px 8px;
    border-radius: 4px;
    font-size: 0.8rem;
  }
  .filters input { flex: 1; }

  ul { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 4px; }

  .run-row { display: flex; align-items: center; gap: 8px; }
  .check { accent-color: #4FC3F7; cursor: pointer; }

  .run-item {
    flex: 1;
    min-width: 0;
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
  .flag { color: #FFD54F; margin-left: 6px; }
  .run-tags { display: flex; gap: 4px; flex-wrap: wrap; margin-top: 5px; padding-right: 20px; }
  .tag {
    font-size: 0.68rem;
    color: #9ab;
    background: #1a1a38;
    border: 1px solid #2a2a4a;
    border-radius: 8px;
    padding: 0 7px;
  }
  .notes {
    font-size: 0.7rem;
    color: #666;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 320px;
  }
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
  .btn-compare {
    background: #4FC3F7;
    color: #000;
    border: none;
    padding: 3px 10px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.8rem;
    font-weight: 600;
  }
  .btn-compare:disabled { background: #1e1e38; color: #555; cursor: default; }
</style>
