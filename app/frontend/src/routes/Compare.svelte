<script>
  import { onMount } from 'svelte';
  import { writable } from 'svelte/store';
  import { apiGet } from '../lib/ws.js';
  import { CHANNELS, GROUPS, ALL_CHANNEL_KEYS } from '../lib/channels.js';
  import { loadRunData, fetchMarkers, nearestIdx, fmtRel, fmtValue, runDuration } from '../lib/runs.js';
  import { alignmentOffset, buildOverlay, rangeStats } from '../lib/compare.js';
  import { compareSelection } from '../lib/nav.js';
  import CompareChart from '../lib/components/CompareChart.svelte';

  const MAX_RUNS = 4;
  const RUN_COLORS = ['#4FC3F7', '#FFB74D', '#81C784', '#F06292'];
  const CHANNELS_KEY = 'telemetry-compare-channels';

  let runs = [];
  let error = '';
  let selected = [...$compareSelection].slice(0, MAX_RUNS);
  let channels = loadChannels();
  let align = { mode: 'start', label: '', channel: 'tps1', value: 50 };

  // runId → {samples, markers} | 'loading' | {error}
  let cache = {};

  const xRange = writable(null);
  let cursorX = null;
  let chartW = 600;
  let showChannelPicker = false;

  function loadChannels() {
    try {
      const saved = JSON.parse(localStorage.getItem(CHANNELS_KEY));
      if (Array.isArray(saved) && saved.every(k => CHANNELS[k])) return saved;
    } catch {}
    return ['tps1', 'torque_cmd_l', 'torque_cmd_r'];
  }

  function saveChannels() {
    try { localStorage.setItem(CHANNELS_KEY, JSON.stringify(channels)); } catch {}
  }

  onMount(async () => {
    try {
      runs = await apiGet('/api/runs');
    } catch (e) {
      error = e.message;
    }
  });

  async function ensureLoaded(id) {
    if (cache[id]) return;
    cache[id] = 'loading';
    try {
      const [data, markers] = await Promise.all([loadRunData(id), fetchMarkers(id)]);
      cache[id] = { samples: data.samples, markers };
    } catch (e) {
      cache[id] = { error: e.message };
    }
  }

  $: selected.forEach(ensureLoaded);
  $: compareSelection.set(selected);

  function toggleRun(id) {
    if (selected.includes(id)) selected = selected.filter(x => x !== id);
    else if (selected.length < MAX_RUNS) selected = [...selected, id];
    xRange.set(null);
  }

  function toggleChannel(key) {
    channels = channels.includes(key) ? channels.filter(k => k !== key) : [...channels, key];
    saveChannels();
  }

  // ── alignment and overlay ───────────────────────────────────────────────────
  $: runById = Object.fromEntries(runs.map(r => [r.id, r]));
  $: loaded = selected
    .map((id, i) => ({ id, color: RUN_COLORS[i], run: runById[id], data: cache[id] }))
    .filter(e => e.run && e.data?.samples);
  $: loadingAny = selected.some(id => cache[id] === 'loading');

  $: markerLabels = [...new Set(loaded.flatMap(e => e.data.markers.map(m => m.label)))].filter(Boolean);
  $: if (align.mode === 'marker' && !markerLabels.includes(align.label)) align.label = markerLabels[0] ?? '';

  $: entries = loaded.map(e => ({
    ...e,
    samples: e.data.samples,
    ...alignmentOffset(e.data.samples, e.data.markers, align),
  }));
  $: notAligned = entries.filter(e => !e.found);

  $: overlay = buildOverlay(entries, channels);
  $: charts = channels.map(key => ({
    key,
    meta: CHANNELS[key],
    series: entries.map((e, ri) => ({
      label: e.run.name,
      color: e.color,
      values: overlay.byChannel[key][ri],
    })),
  }));
  $: markerLines = entries.flatMap(e =>
    e.data.markers.map(m => ({ x: m.ts - e.offset, color: e.color, label: m.label }))
  );

  // Changing the alignment moves every run, so the old zoom no longer applies.
  $: align, xRange.set(null);

  // ── tables ──────────────────────────────────────────────────────────────────
  function valueAt(ri, key, x) {
    const rel = overlay.rel[ri];
    if (x == null || !rel?.length) return null;
    const i = nearestIdx(rel, x);
    if (Math.abs(rel[i] - x) > 0.25) return null;  // outside this run
    return entries[ri].samples[ALL_CHANNEL_KEYS.indexOf(key) + 1][i];
  }

  $: lo = $xRange?.min ?? -Infinity;
  $: hi = $xRange?.max ?? Infinity;
  $: stats = channels.map(key => ({
    key,
    perRun: entries.map((e, ri) =>
      rangeStats(overlay.rel[ri], e.samples[ALL_CHANNEL_KEYS.indexOf(key) + 1], lo, hi)
    ),
  }));

  function fmtDate(ts) {
    return new Date(ts * 1000).toLocaleString([], { dateStyle: 'short', timeStyle: 'short' });
  }
</script>

<div class="compare">
  <!-- config column ───────────────────────────────────────────────────────── -->
  <aside class="config">
    <section>
      <h3>Runs <span class="dim">({selected.length}/{MAX_RUNS})</span></h3>
      {#if error}<p class="err">{error}</p>{/if}
      <div class="run-list">
        {#each runs as r (r.id)}
          {@const i = selected.indexOf(r.id)}
          <label class="run-opt" class:on={i >= 0}>
            <input
              type="checkbox"
              checked={i >= 0}
              disabled={i < 0 && selected.length >= MAX_RUNS}
              on:change={() => toggleRun(r.id)}
            />
            <span class="swatch" style="background: {i >= 0 ? RUN_COLORS[i] : 'transparent'}"></span>
            <span class="run-text">
              <span class="run-name">{r.name}</span>
              <span class="dim">{fmtDate(r.started_at)} · {runDuration(r)}{r.driver ? ` · ${r.driver}` : ''}{r.mode ? ` · ${r.mode}` : ''}</span>
            </span>
            {#if cache[r.id] === 'loading'}<span class="dim">…</span>{/if}
            {#if cache[r.id]?.error}<span class="err" title={cache[r.id].error}>!</span>{/if}
          </label>
        {/each}
      </div>
    </section>

    <section>
      <h3>Align on</h3>
      <select bind:value={align.mode}>
        <option value="start">Start of each run</option>
        <option value="marker" disabled={!markerLabels.length}>Marker with label…</option>
        <option value="threshold">First time a channel reaches…</option>
      </select>
      {#if align.mode === 'marker'}
        <select bind:value={align.label}>
          {#each markerLabels as l}<option value={l}>{l}</option>{/each}
        </select>
      {:else if align.mode === 'threshold'}
        <div class="thr">
          <select bind:value={align.channel}>
            {#each ALL_CHANNEL_KEYS as k}<option value={k}>{CHANNELS[k].name}</option>{/each}
          </select>
          <span>≥</span>
          <input type="number" bind:value={align.value} />
        </div>
      {/if}
      {#if notAligned.length}
        <p class="warn">Not found in {notAligned.map(e => e.run.name).join(', ')}; aligned on its start.</p>
      {/if}
    </section>

    <section>
      <h3>
        Channels <span class="dim">({channels.length})</span>
        <button class="link-btn" on:click={() => showChannelPicker = !showChannelPicker}>
          {showChannelPicker ? 'Done' : 'Edit'}
        </button>
      </h3>
      {#if showChannelPicker}
        {#each Object.entries(GROUPS) as [gKey, g] (gKey)}
          <div class="group-label">{g.label}</div>
          <div class="chips">
            {#each g.channels as key (key)}
              <button
                class="chip"
                class:on={channels.includes(key)}
                style="--c: {CHANNELS[key].color}"
                on:click={() => toggleChannel(key)}
              >{CHANNELS[key].name}</button>
            {/each}
          </div>
        {/each}
      {:else}
        <div class="chips">
          {#each channels as key (key)}
            <button class="chip on" style="--c: {CHANNELS[key].color}" on:click={() => toggleChannel(key)} title="Remove">
              {CHANNELS[key].name} ✕
            </button>
          {/each}
        </div>
      {/if}
    </section>
  </aside>

  <!-- charts ──────────────────────────────────────────────────────────────── -->
  <div class="main" bind:clientWidth={chartW}>
    {#if !selected.length}
      <div class="empty">Pick up to {MAX_RUNS} runs on the left, or tick them in the Runs tab and press Compare.</div>
    {:else if !entries.length}
      <div class="empty">{loadingAny ? 'Loading runs…' : 'Selected runs have no samples.'}</div>
    {:else}
      <!-- values at cursor + stats -->
      <div class="tables">
        <table>
          <thead>
            <tr>
              <th class="left">Run</th>
              {#each channels as key}<th>{CHANNELS[key].name} <span class="dim">{CHANNELS[key].unit}</span></th>{/each}
            </tr>
          </thead>
          <tbody>
            <tr class="section"><td colspan={channels.length + 1}>At cursor {cursorX == null ? '' : fmtRel(cursorX)}</td></tr>
            {#each entries as e, ri (e.id)}
              <tr>
                <td class="left"><span class="swatch" style="background: {e.color}"></span>{e.run.name}</td>
                {#each channels as key}<td class="num">{fmtValue(valueAt(ri, key, cursorX))}</td>{/each}
              </tr>
            {/each}
            <tr class="section">
              <td colspan={channels.length + 1}>
                Min / avg / max over {$xRange ? `${fmtRel(lo)} – ${fmtRel(hi)}` : 'whole runs'}
              </td>
            </tr>
            {#each entries as e, ri (e.id)}
              <tr>
                <td class="left"><span class="swatch" style="background: {e.color}"></span>{e.run.name}</td>
                {#each stats as s}
                  {@const st = s.perRun[ri]}
                  <td class="num">{st ? `${fmtValue(st.min)} / ${fmtValue(st.avg)} / ${fmtValue(st.max)}` : '—'}</td>
                {/each}
              </tr>
            {/each}
          </tbody>
        </table>
      </div>

      <div class="charts">
        {#each charts as c (c.key)}
          <CompareChart
            title={c.meta.name}
            unit={c.meta.unit}
            xs={overlay.xs}
            series={c.series}
            markers={markerLines}
            width={Math.max(200, chartW - 40)}
            height={200}
            rangeStore={xRange}
            on:cursor={(e) => cursorX = e.detail}
          />
        {/each}
      </div>
      <p class="hint">Drag on a chart to zoom all of them. Time is relative to each run's alignment point.</p>
    {/if}
  </div>
</div>

<style>
  .compare { display: flex; height: 100%; overflow: hidden; }
  .config {
    width: 290px;
    flex-shrink: 0;
    overflow-y: auto;
    border-right: 1px solid #1e1e38;
    padding: 10px 12px;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  h3 {
    margin: 0 0 6px;
    font-size: 0.72rem;
    color: #888;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .dim { color: #555; font-size: 0.7rem; text-transform: none; letter-spacing: 0; }
  .err { color: #e57373; font-size: 0.75rem; }
  .warn { color: #FFB74D; font-size: 0.72rem; margin: 6px 0 0; }

  .run-list { display: flex; flex-direction: column; gap: 3px; max-height: 300px; overflow-y: auto; }
  .run-opt {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 6px;
    border: 1px solid #1e1e38;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.76rem;
  }
  .run-opt.on { border-color: #3a3a5a; background: #16162e; }
  .run-text { display: flex; flex-direction: column; min-width: 0; flex: 1; }
  .run-name { color: #ddd; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .swatch { display: inline-block; width: 10px; height: 10px; border-radius: 2px; flex-shrink: 0; margin-right: 4px; border: 1px solid #333; }

  select, input[type="number"] {
    width: 100%;
    background: #0f0f1e;
    border: 1px solid #2a2a4a;
    color: #e0e0e0;
    padding: 4px 6px;
    border-radius: 4px;
    font-size: 0.78rem;
    margin-bottom: 4px;
  }
  .thr { display: flex; align-items: center; gap: 6px; color: #888; }
  .thr select { flex: 1; }
  .thr input { width: 70px; }

  .link-btn { margin-left: auto; background: none; border: none; color: #4FC3F7; cursor: pointer; font-size: 0.72rem; }
  .group-label { font-size: 0.64rem; color: #555; text-transform: uppercase; margin: 6px 0 3px; }
  .chips { display: flex; flex-wrap: wrap; gap: 4px; }
  .chip {
    background: #0f0f1e;
    border: 1px solid #2a2a4a;
    color: #666;
    border-radius: 3px;
    padding: 2px 6px;
    font-size: 0.7rem;
    cursor: pointer;
  }
  .chip.on { border-color: var(--c); color: var(--c); }

  .main { flex: 1; overflow-y: auto; padding: 10px 12px; min-width: 0; }
  .empty { color: #555; font-size: 0.85rem; padding: 40px; text-align: center; }

  .tables { overflow-x: auto; margin-bottom: 10px; }
  table { border-collapse: collapse; font-size: 0.74rem; width: 100%; }
  th, td { padding: 3px 8px; border-bottom: 1px solid #1a1a32; white-space: nowrap; }
  th { color: #888; font-weight: 600; text-align: right; }
  .left { text-align: left; }
  td.left { color: #ccc; max-width: 220px; overflow: hidden; text-overflow: ellipsis; }
  td.num { text-align: right; font-family: monospace; color: #ddd; }
  tr.section td { color: #555; font-size: 0.66rem; text-transform: uppercase; letter-spacing: 0.05em; padding-top: 8px; }

  .charts { display: flex; flex-direction: column; gap: 8px; }
  .hint { color: #555; font-size: 0.7rem; margin-top: 8px; }
</style>
