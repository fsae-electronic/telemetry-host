<script>
  import { setContext, onDestroy } from 'svelte';
  import { writable } from 'svelte/store';
  import { valuesAtIndex } from '../lib/playback.js';
  import { dashboard } from '../lib/dashboard.js';
  import {
    loadRunData, fetchMarkers, addMarker, updateMarker, deleteMarker,
    nearestIdx, fmtRel, runDuration, META_FIELDS,
  } from '../lib/runs.js';
  import DashboardGrid from '../lib/components/DashboardGrid.svelte';
  import RunSidebar    from '../lib/components/RunSidebar.svelte';
  import ExportModal   from '../lib/components/ExportModal.svelte';

  export let run;
  export let onBack;
  export let onUpdated = () => {};   // called with the run after its metadata is saved
  export let suggestions = {};       // metadata values used in other runs

  // ── context stores ──────────────────────────────────────────────────────────
  const samplesStore = writable(null);
  const valuesStore  = writable({});
  const cursorTs     = writable(null);   // shared timestamp for all panels
  const markersStore = writable([]);     // this run's markers, sorted by ts
  const xRange       = writable(null);   // shared zoom {min, max}, null = whole run

  // Seek to exact sample index (used by scrubber + playback timer)
  function scrubTo(idx) {
    const s = $samplesStore;
    if (!s) return;
    scrubIdx = Math.max(0, Math.min(idx, s[0].length - 1));
    valuesStore.set(valuesAtIndex(s, scrubIdx));
    cursorTs.set(s[0][scrubIdx]);
  }

  // Seek to a timestamp (called by chart cursor hover/click)
  function seekToTs(ts) {
    const s = $samplesStore;
    if (!s?.[0]?.length) return;
    scrubTo(nearestIdx(s[0], ts));
  }

  setContext('datasource', {
    mode: 'playback',
    samplesStore,
    valuesStore,
    cursorTs,
    seekToTs,
    markersStore,
    xRange,
  });

  // ── load ────────────────────────────────────────────────────────────────────
  let loading = true;
  let error   = '';
  let totalSamples = 0;
  let events = [];

  async function load() {
    loading = true; error = '';
    try {
      const [data, markers] = await Promise.all([loadRunData(run.id), fetchMarkers(run.id)]);
      if (!data.samples[0].length) { error = 'No samples in this run.'; return; }
      totalSamples = data.samples[0].length;
      events = data.events;
      markersStore.set(markers);
      samplesStore.set(data.samples);
      scrubTo(0);
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  // ── scrubber state ──────────────────────────────────────────────────────────
  let scrubIdx = 0;
  $: samples   = $samplesStore;
  $: maxIdx    = Math.max(0, totalSamples - 1);
  $: t0        = samples?.[0]?.[0] ?? run.started_at;
  $: tEnd      = samples?.[0]?.[maxIdx] ?? t0;

  // ── markers ─────────────────────────────────────────────────────────────────
  function setMarkers(list) {
    markersStore.set([...list].sort((a, b) => a.ts - b.ts));
  }

  async function addMarkerAtCursor() {
    const ts = $cursorTs;
    if (ts == null) return;
    try {
      const m = await addMarker(run.id, `Marker ${$markersStore.length + 1}`, ts);
      setMarkers([...$markersStore, m]);
      onUpdated({ ...run, markers: $markersStore.length });
    } catch (e) {
      error = e.message;
    }
  }

  async function onRenameMarker(e) {
    const { id, label } = e.detail;
    try {
      await updateMarker(run.id, id, { label });
      setMarkers($markersStore.map(m => m.id === id ? { ...m, label } : m));
    } catch (err) {
      error = err.message;
    }
  }

  async function onDeleteMarker(e) {
    try {
      await deleteMarker(run.id, e.detail);
      setMarkers($markersStore.filter(m => m.id !== e.detail));
      onUpdated({ ...run, markers: $markersStore.length });
    } catch (err) {
      error = err.message;
    }
  }

  // ── playback ────────────────────────────────────────────────────────────────
  const SPEEDS = [0.25, 0.5, 1, 2, 4];
  let playing        = false;
  let playSpeed      = 1;
  let rafHandle      = null;
  let playStartReal  = 0;   // performance.now() when play started
  let playStartRunTs = 0;   // run timestamp when play started

  function startPlay() {
    if (!samples || scrubIdx >= maxIdx) scrubTo(0);
    playStartReal  = performance.now();
    playStartRunTs = samples?.[0]?.[scrubIdx] ?? 0;
    playing = true;

    function tick() {
      if (!playing || !samples) return;
      const realElapsed = (performance.now() - playStartReal) / 1000;
      const targetTs    = playStartRunTs + realElapsed * playSpeed;
      const idx         = nearestIdx(samples[0], targetTs);
      scrubTo(idx);
      if (idx >= maxIdx) { stopPlay(); return; }
      rafHandle = requestAnimationFrame(tick);
    }
    rafHandle = requestAnimationFrame(tick);
  }

  function stopPlay() {
    playing = false;
    if (rafHandle) { cancelAnimationFrame(rafHandle); rafHandle = null; }
  }

  function togglePlay() {
    if (playing) stopPlay(); else startPlay();
  }

  function setSpeed(s) {
    const wasPlaying = playing;
    if (wasPlaying) stopPlay();
    playSpeed = s;
    if (wasPlaying) startPlay();
  }

  function restart() {
    stopPlay();
    scrubTo(0);
  }

  onDestroy(stopPlay);

  // ── keyboard ────────────────────────────────────────────────────────────────
  // M marker at cursor, Space play/pause, ←/→ one sample (Shift: ten).
  function onKey(e) {
    if (loading || error || showExport) return;
    if (e.target.closest?.('input, textarea, select, [contenteditable]')) return;
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    if (e.key === 'm' || e.key === 'M') {
      addMarkerAtCursor();
    } else if (e.key === ' ') {
      e.preventDefault();
      togglePlay();
    } else if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
      e.preventDefault();
      stopPlay();
      scrubTo(scrubIdx + (e.key === 'ArrowLeft' ? -1 : 1) * (e.shiftKey ? 10 : 1));
    }
  }

  // ── header / sidebar ────────────────────────────────────────────────────────
  let showSidebar = true;
  let showExport  = false;

  $: metaChips = META_FIELDS.map(f => run[f.key]).filter(Boolean);
  $: panelChannels = [...new Set($dashboard.flatMap(p => p.channels ?? []))];

  function onSaved(e) {
    run = { ...run, ...e.detail };
    onUpdated(run);
  }

  // ── helpers ─────────────────────────────────────────────────────────────────
  function fmtClock(ts) {
    if (!ts) return '';
    const d = new Date(ts * 1000);
    return `${d.getHours().toString().padStart(2,'0')}:${d.getMinutes().toString().padStart(2,'0')}:${d.getSeconds().toString().padStart(2,'0')}.${Math.floor(d.getMilliseconds()/100)}`;
  }

  function tickPos(ts) {
    return tEnd > t0 ? ((ts - t0) / (tEnd - t0)) * 100 : 0;
  }

  load();
</script>

<svelte:window on:keydown={onKey} />

<div class="run-dash">

  <!-- header ────────────────────────────────────────────────────────────────── -->
  <div class="run-header">
    <button class="back-btn" on:click={onBack}>← Runs</button>
    <div class="run-info">
      <span class="run-name">{run.name}</span>
      <span class="run-meta">
        {runDuration(run)}{totalSamples ? ` · ${totalSamples} samples` : ''}
        {#each metaChips as c}<span class="chip">{c}</span>{/each}
      </span>
    </div>
    <div class="header-actions">
      <button class="hdr-btn" on:click={() => showExport = true} disabled={!samples}>⤓ Export CSV</button>
      <button class="hdr-btn" class:active={showSidebar} on:click={() => showSidebar = !showSidebar}>☰ Sidebar</button>
    </div>
  </div>

  <!-- grid ──────────────────────────────────────────────────────────────────── -->
  {#if loading}
    <div class="state-msg">Loading samples…</div>
  {:else if error && !samples}
    <div class="state-msg err">{error}</div>
  {:else}
    <div class="content">
      <div class="grid-scroll">
        {#if error}<div class="inline-err">{error} <button on:click={() => error = ''}>✕</button></div>{/if}
        <DashboardGrid editMode={false} />
      </div>
      {#if showSidebar}
        <RunSidebar
          {run}
          {suggestions}
          {events}
          {t0}
          values={$valuesStore}
          cursorTs={$cursorTs}
          markers={$markersStore}
          on:seek={(e) => { stopPlay(); seekToTs(e.detail); }}
          on:addMarker={addMarkerAtCursor}
          on:renameMarker={onRenameMarker}
          on:deleteMarker={onDeleteMarker}
          on:saved={onSaved}
        />
      {/if}
    </div>

    <!-- transport bar ───────────────────────────────────────────────────────── -->
    <div class="transport">

      <!-- playback controls -->
      <div class="controls">
        <button class="ctrl-btn" on:click={restart} title="Restart">⏮</button>
        <button class="ctrl-btn play" on:click={togglePlay} title={playing ? 'Pause (Space)' : 'Play (Space)'}>
          {playing ? '⏸' : '▶'}
        </button>
        <button class="ctrl-btn marker" on:click={addMarkerAtCursor} title="Add marker at cursor (M)">⚑</button>
      </div>

      <!-- speed pills -->
      <div class="speeds">
        {#each SPEEDS as s}
          <button
            class="speed-pill"
            class:active={playSpeed === s}
            on:click={() => setSpeed(s)}
          >{s}×</button>
        {/each}
      </div>

      <!-- scrubber -->
      <span class="ts-edge">{fmtClock(t0)}</span>
      <div class="scrub-wrap">
        <input
          class="scrub-track"
          type="range"
          min="0"
          max={maxIdx}
          step="1"
          bind:value={scrubIdx}
          on:input={() => { stopPlay(); scrubTo(scrubIdx); }}
        />
        {#each $markersStore as m (m.id)}
          <button
            class="scrub-marker"
            style="left: {tickPos(m.ts)}%"
            title="{fmtRel(m.ts - t0)} {m.label}"
            on:click={() => { stopPlay(); seekToTs(m.ts); }}
          ></button>
        {/each}
      </div>
      <span class="ts-edge">{fmtClock(tEnd)}</span>

      <!-- current timestamp -->
      <span class="ts-now" class:playing>
        {fmtRel(($cursorTs ?? t0) - t0)}
        <span class="ts-clock">{fmtClock($cursorTs)}</span>
      </span>

    </div>
  {/if}

</div>

{#if showExport && samples}
  <ExportModal
    {run}
    {samples}
    {panelChannels}
    markers={$markersStore}
    zoom={$xRange}
    on:close={() => showExport = false}
  />
{/if}

<style>
  .run-dash {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
  }

  /* header */
  .run-header {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 6px 14px;
    background: #141428;
    border-bottom: 1px solid #2a2a4a;
    flex-shrink: 0;
  }
  .back-btn {
    background: none;
    border: 1px solid #2a2a4a;
    color: #4FC3F7;
    padding: 3px 10px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.78rem;
  }
  .back-btn:hover { background: #1a1a38; }
  .run-info { display: flex; flex-direction: column; gap: 1px; flex: 1; min-width: 0; }
  .run-name { font-size: 0.88rem; color: #e0e0e0; font-weight: 600; }
  .run-meta { font-size: 0.72rem; color: #555; display: flex; gap: 6px; align-items: center; flex-wrap: wrap; }
  .chip {
    background: #1e1e38;
    border: 1px solid #2a2a4a;
    border-radius: 8px;
    padding: 0 7px;
    color: #9aa;
  }
  .header-actions { display: flex; gap: 6px; }
  .hdr-btn {
    background: #1a1a38;
    border: 1px solid #2a2a4a;
    color: #aaa;
    padding: 4px 10px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.78rem;
    font-weight: 600;
  }
  .hdr-btn:hover { border-color: #4FC3F7; color: #4FC3F7; }
  .hdr-btn.active { color: #4FC3F7; }
  .hdr-btn:disabled { opacity: 0.4; cursor: default; }

  /* grid + sidebar */
  .content { flex: 1; display: flex; min-height: 0; }
  .grid-scroll { flex: 1; overflow-y: auto; min-height: 0; min-width: 0; }
  .inline-err {
    margin: 8px 8px 0;
    padding: 6px 10px;
    border: 1px solid #5c1f1f;
    background: #2a1212;
    color: #e57373;
    border-radius: 4px;
    font-size: 0.78rem;
    display: flex;
    justify-content: space-between;
  }
  .inline-err button { background: none; border: none; color: #e57373; cursor: pointer; }

  /* transport bar */
  .transport {
    flex-shrink: 0;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 5px 12px;
    background: #0a0a1a;
    border-top: 1px solid #1e1e38;
    height: 42px;
  }

  .controls { display: flex; gap: 4px; }
  .ctrl-btn {
    background: #1a1a38;
    border: 1px solid #2a2a4a;
    color: #aaa;
    width: 28px;
    height: 28px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.8rem;
    display: flex; align-items: center; justify-content: center;
    transition: all 0.1s;
  }
  .ctrl-btn:hover { border-color: #4FC3F7; color: #4FC3F7; }
  .ctrl-btn.play { color: #e0e0e0; }
  .ctrl-btn.marker { color: #FFD54F; }
  .ctrl-btn.marker:hover { border-color: #FFD54F; }

  .speeds { display: flex; gap: 2px; }
  .speed-pill {
    background: none;
    border: 1px solid #1e1e38;
    color: #444;
    padding: 1px 6px;
    border-radius: 3px;
    cursor: pointer;
    font-size: 0.68rem;
    font-family: monospace;
    transition: all 0.1s;
    height: 22px;
  }
  .speed-pill:hover  { border-color: #3a3a5a; color: #888; }
  .speed-pill.active { border-color: #4FC3F7; color: #4FC3F7; background: rgba(79,195,247,0.1); }

  .scrub-wrap { flex: 1; position: relative; display: flex; align-items: center; }
  .scrub-track {
    flex: 1;
    accent-color: #4FC3F7;
    cursor: pointer;
    height: 4px;
  }
  .scrub-marker {
    position: absolute;
    top: -9px;
    width: 3px;
    height: 9px;
    margin-left: -1px;
    padding: 0;
    border: none;
    background: #FFD54F;
    cursor: pointer;
  }
  .ts-edge {
    font-size: 0.65rem;
    color: #444;
    font-family: monospace;
    white-space: nowrap;
  }
  .ts-now {
    font-size: 0.78rem;
    color: #888;
    font-family: monospace;
    white-space: nowrap;
    min-width: 150px;
    text-align: right;
    transition: color 0.1s;
  }
  .ts-now.playing { color: #81C784; }
  .ts-clock { color: #444; font-size: 0.68rem; margin-left: 4px; }

  /* states */
  .state-msg {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    color: #444;
    font-size: 0.85rem;
  }
  .state-msg.err { color: #e57373; }
</style>
