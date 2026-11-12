<script>
  import { createEventDispatcher } from 'svelte';
  import { ALL_CHANNEL_KEYS, RAW_CHANNEL_KEYS } from '../channels.js';
  import { buildCsv, downloadText, safeFilename } from '../csv.js';
  import { fmtRel } from '../runs.js';

  export let run;
  export let samples;           // uPlot-format run data
  export let markers = [];
  export let zoom = null;       // {min, max} of the shared chart zoom, if any
  export let panelChannels = [];// channels shown on the dashboard

  const dispatch = createEventDispatcher();
  const t0 = samples[0][0];

  let range = zoom ? 'zoom' : 'all';
  let fromMarker = markers[0]?.id;
  let toMarker = markers[1]?.id ?? markers[0]?.id;
  let channelSet = 'all';
  let excel = false;

  $: bounds = (() => {
    if (range === 'zoom' && zoom) return { from: zoom.min, to: zoom.max };
    if (range === 'markers') {
      const a = markers.find(m => m.id === fromMarker)?.ts;
      const b = markers.find(m => m.id === toMarker)?.ts;
      if (a != null && b != null) return { from: Math.min(a, b), to: Math.max(a, b) };
    }
    return { from: -Infinity, to: Infinity };
  })();

  $: keys = channelSet === 'raw' ? RAW_CHANNEL_KEYS
    : channelSet === 'dashboard' ? ALL_CHANNEL_KEYS.filter(k => panelChannels.includes(k))
    : ALL_CHANNEL_KEYS;

  $: rowCount = samples[0].filter(t => t >= bounds.from && t <= bounds.to).length;

  function exportCsv() {
    const { text } = buildCsv({
      samples, keys, excel,
      markers: markers.filter(m => m.ts >= bounds.from && m.ts <= bounds.to),
      from: bounds.from, to: bounds.to,
    });
    downloadText(`${safeFilename(run.name)}.csv`, text, { bom: excel });
    dispatch('close');
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="overlay" on:click|self={() => dispatch('close')}>
  <div class="modal" role="dialog" aria-modal="true">
    <div class="modal-header">
      <h3>Export CSV</h3>
      <button class="close" on:click={() => dispatch('close')}>✕</button>
    </div>

    <div class="modal-body">
      <fieldset>
        <legend>Range</legend>
        <label><input type="radio" bind:group={range} value="all" /> Whole run</label>
        <label class:disabled={!zoom}>
          <input type="radio" bind:group={range} value="zoom" disabled={!zoom} />
          Current zoom
          {#if zoom}<span class="hint">{fmtRel(zoom.min - t0)} – {fmtRel(zoom.max - t0)}</span>{/if}
        </label>
        <label class:disabled={markers.length < 2}>
          <input type="radio" bind:group={range} value="markers" disabled={markers.length < 2} />
          Between markers
        </label>
        {#if range === 'markers'}
          <div class="marker-pick">
            <select bind:value={fromMarker}>
              {#each markers as m}<option value={m.id}>{fmtRel(m.ts - t0)} {m.label}</option>{/each}
            </select>
            →
            <select bind:value={toMarker}>
              {#each markers as m}<option value={m.id}>{fmtRel(m.ts - t0)} {m.label}</option>{/each}
            </select>
          </div>
        {/if}
      </fieldset>

      <fieldset>
        <legend>Channels</legend>
        <label><input type="radio" bind:group={channelSet} value="all" /> All, including calculated ({ALL_CHANNEL_KEYS.length})</label>
        <label><input type="radio" bind:group={channelSet} value="raw" /> Only from the car ({RAW_CHANNEL_KEYS.length})</label>
        <label class:disabled={!panelChannels.length}>
          <input type="radio" bind:group={channelSet} value="dashboard" disabled={!panelChannels.length} />
          Shown on the dashboard ({ALL_CHANNEL_KEYS.filter(k => panelChannels.includes(k)).length})
        </label>
      </fieldset>

      <fieldset>
        <legend>Format</legend>
        <label><input type="radio" bind:group={excel} value={false} /> Standard (<code>,</code> separator, <code>.</code> decimals) — Python, MATLAB</label>
        <label><input type="radio" bind:group={excel} value={true} /> Excel in Spanish (<code>;</code> separator, <code>,</code> decimals)</label>
      </fieldset>

      <p class="hint">
        {rowCount} rows × {keys.length + 3} columns. Columns: <code>time_s</code> (seconds since run start),
        <code>timestamp</code> (Unix seconds), <code>marker</code>, then one per channel with raw values.
      </p>
    </div>

    <div class="modal-footer">
      <button class="btn-ghost" on:click={() => dispatch('close')}>Cancel</button>
      <button class="btn-primary" on:click={exportCsv} disabled={!rowCount}>Download</button>
    </div>
  </div>
</div>

<style>
  .overlay {
    position: fixed; inset: 0; background: rgba(0,0,0,0.7);
    display: flex; align-items: center; justify-content: center; z-index: 100;
  }
  .modal {
    background: #1a1a38; border: 1px solid #3a3a5a; border-radius: 8px;
    width: 480px; max-width: 95vw; max-height: 90vh;
    display: flex; flex-direction: column; overflow: hidden;
  }
  .modal-header {
    display: flex; justify-content: space-between; align-items: center;
    padding: 12px 16px; border-bottom: 1px solid #2a2a4a;
  }
  h3 { margin: 0; font-size: 1rem; color: #e0e0e0; }
  .close { background: none; border: none; color: #666; font-size: 1rem; cursor: pointer; }
  .close:hover { color: #e57373; }
  .modal-body { padding: 14px 16px; overflow-y: auto; display: flex; flex-direction: column; gap: 12px; }
  .modal-footer {
    padding: 12px 16px; border-top: 1px solid #2a2a4a;
    display: flex; justify-content: flex-end; gap: 8px;
  }
  fieldset { border: 1px solid #2a2a4a; border-radius: 6px; padding: 8px 12px; margin: 0; display: flex; flex-direction: column; gap: 5px; }
  legend { font-size: 0.7rem; color: #888; text-transform: uppercase; letter-spacing: 0.05em; padding: 0 4px; }
  label { font-size: 0.8rem; color: #ccc; display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
  label.disabled { color: #555; }
  .marker-pick { display: flex; align-items: center; gap: 6px; margin-left: 22px; font-size: 0.8rem; color: #888; }
  select {
    background: #0f0f1e; border: 1px solid #3a3a5a; color: #e0e0e0;
    padding: 3px 6px; border-radius: 4px; font-size: 0.78rem; max-width: 170px;
  }
  .hint { font-size: 0.72rem; color: #777; margin: 0; }
  code { font-size: 0.72rem; color: #4FC3F7; }
  .btn-primary { background: #4FC3F7; color: #000; border: none; padding: 6px 16px; border-radius: 4px; cursor: pointer; font-weight: 600; }
  .btn-primary:disabled { opacity: 0.5; cursor: default; }
  .btn-ghost { background: none; border: 1px solid #3a3a5a; color: #aaa; padding: 6px 16px; border-radius: 4px; cursor: pointer; }
</style>
