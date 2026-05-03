<script>
  import { createEventDispatcher } from 'svelte';
  import { CHANNELS, GROUPS } from '../channels.js';
  import { PANEL_TYPES } from '../dashboard.js';

  export let panel = null;   // null = add mode, object = edit mode

  const dispatch = createEventDispatcher();

  // Local editable copy
  let title    = panel?.title    ?? '';
  let type     = panel?.type     ?? 'timeseries';
  let selected = new Set(panel?.channels ?? []);
  let min      = panel?.min      ?? -150;
  let max      = panel?.max      ?? 150;
  let cols     = panel?.cols     ?? 6;
  let rows     = panel?.rows     ?? 4;

  const ALL_KEYS = Object.keys(CHANNELS);

  function toggleChannel(key) {
    if (selected.has(key)) selected.delete(key);
    else selected.add(key);
    selected = new Set(selected);
  }

  function selectGroup(groupKey) {
    for (const key of GROUPS[groupKey].channels) selected.add(key);
    selected = new Set(selected);
  }

  function save() {
    if (!title.trim()) { alert('Panel needs a title.'); return; }
    const channels = [...selected];
    if (channels.length === 0) { alert('Select at least one channel.'); return; }
    dispatch('save', { title, type, channels, min, max, cols, rows });
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="overlay" on:click|self={() => dispatch('close')}>
  <div class="modal" role="dialog" aria-modal="true">
    <div class="modal-header">
      <h3>{panel ? 'Edit Panel' : 'Add Panel'}</h3>
      <button class="close" on:click={() => dispatch('close')}>✕</button>
    </div>

    <div class="modal-body">
      <!-- Title -->
      <label class="field">
        <span>Title</span>
        <input type="text" bind:value={title} placeholder="Panel title" />
      </label>

      <!-- Panel type -->
      <label class="field">
        <span>Type</span>
        <div class="type-grid">
          {#each Object.entries(PANEL_TYPES) as [key, pt]}
            <button
              class="type-btn"
              class:active={type === key}
              on:click={() => type = key}
            >
              <span class="type-icon">{pt.icon}</span>
              <span class="type-label">{pt.label}</span>
            </button>
          {/each}
        </div>
      </label>

      <!-- Channel picker -->
      <div class="field">
        <span>Channels</span>

        <!-- Quick-select by group -->
        <div class="group-row">
          {#each Object.entries(GROUPS) as [gKey, g]}
            <button class="group-btn" on:click={() => selectGroup(gKey)}>{g.label}</button>
          {/each}
          <button class="group-btn danger" on:click={() => { selected = new Set(); }}>Clear</button>
        </div>

        <div class="channel-grid">
          {#each ALL_KEYS as key}
            {@const meta = CHANNELS[key]}
            <button
              class="ch-btn"
              class:active={selected.has(key)}
              style="--c: {meta.color}"
              on:click={() => toggleChannel(key)}
            >{meta.name}</button>
          {/each}
        </div>

        {#if type === 'scatter'}
          <p class="hint">Scatter expects exactly 2 channels (X, Y in order).</p>
        {/if}
      </div>

      <!-- Range (for gauge / bar) -->
      {#if type === 'gauge' || type === 'bargauge'}
        <div class="field range-row">
          <span>Range</span>
          <label>Min <input type="number" bind:value={min} /></label>
          <label>Max <input type="number" bind:value={max} /></label>
        </div>
      {/if}

      <!-- Size -->
      <div class="field range-row">
        <span>Size (grid units)</span>
        <label>Width&nbsp;<input type="number" bind:value={cols} min="1" max="12" /></label>
        <label>Height&nbsp;<input type="number" bind:value={rows} min="1" max="20" /></label>
      </div>
    </div>

    <div class="modal-footer">
      <button class="btn-ghost" on:click={() => dispatch('close')}>Cancel</button>
      <button class="btn-primary" on:click={save}>{panel ? 'Update' : 'Add Panel'}</button>
    </div>
  </div>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0,0,0,0.7);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
  }
  .modal {
    background: #1a1a38;
    border: 1px solid #3a3a5a;
    border-radius: 8px;
    width: 520px;
    max-width: 95vw;
    max-height: 90vh;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 12px 16px;
    border-bottom: 1px solid #2a2a4a;
  }
  h3 { margin: 0; font-size: 1rem; color: #e0e0e0; }
  .close { background: none; border: none; color: #666; font-size: 1rem; cursor: pointer; }
  .close:hover { color: #e57373; }

  .modal-body {
    padding: 16px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .modal-footer {
    padding: 12px 16px;
    border-top: 1px solid #2a2a4a;
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }

  .field { display: flex; flex-direction: column; gap: 6px; }
  .field > span { font-size: 0.75rem; color: #888; text-transform: uppercase; letter-spacing: 0.05em; }

  input[type="text"], input[type="number"] {
    background: #0f0f1e;
    border: 1px solid #3a3a5a;
    color: #e0e0e0;
    padding: 6px 10px;
    border-radius: 4px;
    font-size: 0.85rem;
  }
  input[type="number"] { width: 70px; }

  .type-grid {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
  }
  .type-btn {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    padding: 8px 12px;
    background: #0f0f1e;
    border: 1px solid #2a2a4a;
    border-radius: 6px;
    color: #888;
    cursor: pointer;
    min-width: 72px;
    transition: border-color 0.1s;
  }
  .type-btn.active { border-color: #4FC3F7; color: #4FC3F7; }
  .type-btn:hover { border-color: #5a5a8a; }
  .type-icon { font-size: 1.2rem; }
  .type-label { font-size: 0.7rem; }

  .group-row { display: flex; gap: 6px; flex-wrap: wrap; margin-bottom: 6px; }
  .group-btn {
    padding: 3px 10px;
    background: #0f0f1e;
    border: 1px solid #2a2a4a;
    color: #aaa;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.75rem;
  }
  .group-btn:hover { border-color: #4FC3F7; color: #4FC3F7; }
  .group-btn.danger:hover { border-color: #e57373; color: #e57373; }

  .channel-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
    gap: 6px;
  }
  .ch-btn {
    padding: 5px 8px;
    background: #0f0f1e;
    border: 1px solid #2a2a4a;
    color: #666;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.75rem;
    text-align: left;
    transition: all 0.1s;
  }
  .ch-btn.active {
    background: var(--c, #4FC3F7)22;
    border-color: var(--c, #4FC3F7);
    color: var(--c, #4FC3F7);
  }
  .ch-btn:hover:not(.active) { border-color: #4a4a6a; color: #aaa; }

  .range-row { flex-direction: row; align-items: center; flex-wrap: wrap; gap: 12px; }
  .range-row > span { width: 100%; }

  .hint { font-size: 0.75rem; color: #666; margin: 4px 0 0; }

  .btn-primary { background: #4FC3F7; color: #000; border: none; padding: 6px 16px; border-radius: 4px; cursor: pointer; font-weight: 600; }
  .btn-ghost   { background: none; border: 1px solid #3a3a5a; color: #aaa; padding: 6px 16px; border-radius: 4px; cursor: pointer; }
</style>
