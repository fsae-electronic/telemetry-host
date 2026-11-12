<script>
  import { createEventDispatcher } from 'svelte';
  import { CHANNELS, GROUPS } from '../channels.js';
  import { fmtRel, fmtValue } from '../runs.js';
  import RunMetaForm from './RunMetaForm.svelte';

  export let run;
  export let values = {};      // channel → value at the cursor
  export let cursorTs = null;
  export let t0 = 0;           // run start, for relative times
  export let markers = [];
  export let events = [];
  export let suggestions = {};

  const dispatch = createEventDispatcher();

  const TABS = [
    ['values', 'Values'],
    ['markers', 'Markers'],
    ['events', 'Events'],
    ['info', 'Info'],
  ];
  let tab = 'values';

  // ── values ────────────────────────────────────────────────────────────────
  let filter = '';
  $: q = filter.trim().toLowerCase();
  $: groups = Object.entries(GROUPS)
    .map(([key, g]) => ({
      key,
      label: g.label,
      channels: g.channels.filter(k => !q || k.includes(q) || CHANNELS[k].name.toLowerCase().includes(q)),
    }))
    .filter(g => g.channels.length);

  // ── events ────────────────────────────────────────────────────────────────
  let levels = { error: true, warn: true, info: false };
  $: shownEvents = events.filter(e => levels[e.level]);
  $: counts = events.reduce((c, e) => ({ ...c, [e.level]: (c[e.level] ?? 0) + 1 }), {});

  function renameMarker(m, label) {
    if (label !== m.label) dispatch('renameMarker', { id: m.id, label });
  }
</script>

<aside class="sidebar">
  <div class="tabs">
    {#each TABS as [key, label]}
      <button class:active={tab === key} on:click={() => tab = key}>
        {label}
        {#if key === 'markers' && markers.length}<span class="count">{markers.length}</span>{/if}
        {#if key === 'events' && counts.error}<span class="count err">{counts.error}</span>{/if}
      </button>
    {/each}
  </div>

  <div class="body">
    {#if tab === 'values'}
      <div class="cursor-time">Cursor <b>{fmtRel(cursorTs == null ? null : cursorTs - t0)}</b></div>
      <input class="filter" type="search" placeholder="Filter channels…" bind:value={filter} />
      {#each groups as g (g.key)}
        <div class="group-label">{g.label}</div>
        {#each g.channels as key (key)}
          {@const meta = CHANNELS[key]}
          <div class="val-row">
            <span class="dot" style="color: {meta.color}">●</span>
            <span class="val-name">{meta.name}</span>
            <span class="val-num">{fmtValue(values[key])}</span>
            <span class="val-unit">{meta.unit}</span>
          </div>
        {/each}
      {/each}

    {:else if tab === 'markers'}
      <button class="add-btn" on:click={() => dispatch('addMarker')}>＋ Marker at cursor <kbd>M</kbd></button>
      {#if !markers.length}
        <p class="muted">No markers. Press <kbd>M</kbd> while recording or here to add one.</p>
      {/if}
      {#each markers as m (m.id)}
        <div class="marker-row">
          <button class="time-btn" on:click={() => dispatch('seek', m.ts)} title="Go to marker">
            {fmtRel(m.ts - t0)}
          </button>
          <input
            class="marker-label"
            value={m.label}
            on:change={(e) => renameMarker(m, e.currentTarget.value)}
            on:keydown={(e) => e.key === 'Enter' && e.currentTarget.blur()}
          />
          <button class="del-btn" title="Delete marker" on:click={() => dispatch('deleteMarker', m.id)}>✕</button>
        </div>
      {/each}

    {:else if tab === 'events'}
      <div class="level-filter">
        {#each ['error', 'warn', 'info'] as lv}
          <label class="lv lv-{lv}">
            <input type="checkbox" bind:checked={levels[lv]} />
            {lv} ({counts[lv] ?? 0})
          </label>
        {/each}
      </div>
      {#if !shownEvents.length}
        <p class="muted">No events{events.length ? ' with these filters' : ' in this run'}.</p>
      {/if}
      {#each shownEvents as e}
        <button class="event-row lv-{e.level}" on:click={() => dispatch('seek', e.ts)}>
          <span class="ev-time">{fmtRel(e.ts - t0)}</span>
          <span class="ev-text">{e.text}</span>
        </button>
      {/each}

    {:else if tab === 'info'}
      {#key run.id}
        <RunMetaForm {run} {suggestions} on:saved />
      {/key}
    {/if}
  </div>
</aside>

<style>
  .sidebar {
    width: 300px;
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    background: #0f0f1e;
    border-left: 1px solid #1e1e38;
    min-height: 0;
  }
  .tabs { display: flex; border-bottom: 1px solid #1e1e38; flex-shrink: 0; }
  .tabs button {
    flex: 1;
    background: none;
    border: none;
    border-bottom: 2px solid transparent;
    color: #666;
    padding: 7px 4px;
    font-size: 0.75rem;
    font-weight: 600;
    cursor: pointer;
  }
  .tabs button:hover { color: #bbb; }
  .tabs button.active { color: #4FC3F7; border-bottom-color: #4FC3F7; }
  .count {
    display: inline-block;
    margin-left: 3px;
    padding: 0 5px;
    border-radius: 8px;
    background: #2a2a4a;
    color: #bbb;
    font-size: 0.65rem;
  }
  .count.err { background: #5c1f1f; color: #ffb4b4; }

  .body { flex: 1; overflow-y: auto; padding: 8px 10px; min-height: 0; }

  .cursor-time { font-size: 0.75rem; color: #777; margin-bottom: 6px; }
  .cursor-time b { color: #e0e0e0; font-family: monospace; }
  .filter {
    width: 100%;
    background: #12122a;
    border: 1px solid #2a2a4a;
    color: #e0e0e0;
    padding: 4px 8px;
    border-radius: 4px;
    font-size: 0.78rem;
    margin-bottom: 6px;
  }
  .group-label {
    font-size: 0.65rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: #555;
    margin: 8px 0 2px;
  }
  .val-row { display: flex; align-items: center; gap: 6px; height: 20px; font-size: 0.75rem; }
  .dot { font-size: 0.55rem; }
  .val-name { flex: 1; color: #aaa; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .val-num { font-family: monospace; color: #e0e0e0; }
  .val-unit { width: 22px; color: #555; font-size: 0.68rem; }

  .add-btn {
    width: 100%;
    background: #1a1a38;
    border: 1px dashed #3a3a5a;
    color: #FFD54F;
    padding: 6px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.78rem;
    margin-bottom: 8px;
  }
  .add-btn:hover { border-color: #FFD54F; }
  kbd {
    font-family: monospace;
    font-size: 0.68rem;
    border: 1px solid #3a3a5a;
    border-radius: 3px;
    padding: 0 4px;
    color: #aaa;
  }
  .marker-row { display: flex; align-items: center; gap: 4px; margin-bottom: 4px; }
  .time-btn {
    background: none;
    border: 1px solid #2a2a4a;
    color: #FFD54F;
    font-family: monospace;
    font-size: 0.72rem;
    padding: 2px 6px;
    border-radius: 3px;
    cursor: pointer;
    min-width: 58px;
  }
  .time-btn:hover { border-color: #FFD54F; }
  .marker-label {
    flex: 1;
    min-width: 0;
    background: #12122a;
    border: 1px solid #2a2a4a;
    color: #e0e0e0;
    padding: 2px 6px;
    border-radius: 3px;
    font-size: 0.78rem;
  }
  .del-btn { background: none; border: none; color: #444; cursor: pointer; font-size: 0.72rem; }
  .del-btn:hover { color: #e57373; }

  .level-filter { display: flex; gap: 10px; margin-bottom: 8px; font-size: 0.72rem; }
  .lv { display: flex; align-items: center; gap: 3px; cursor: pointer; }
  .event-row {
    display: flex;
    gap: 8px;
    width: 100%;
    text-align: left;
    background: none;
    border: none;
    border-left: 2px solid #444;
    padding: 3px 6px;
    margin-bottom: 3px;
    cursor: pointer;
    color: #bbb;
    font-size: 0.74rem;
  }
  .event-row:hover { background: #16162e; }
  .ev-time { font-family: monospace; color: #666; flex-shrink: 0; }
  .lv-error { color: #e57373; }
  .lv-warn  { color: #FFB74D; }
  .lv-info  { color: #90A4AE; }
  .event-row.lv-error { border-left-color: #e57373; color: #f2b8b8; }
  .event-row.lv-warn  { border-left-color: #FFB74D; color: #f5d9b0; }
  .event-row.lv-info  { border-left-color: #455A64; color: #b0bec5; }

  .muted { color: #555; font-size: 0.78rem; }
</style>
