<script>
  import { status, apiPost, apiGet, liveMarkers } from '../ws.js';
  import { addMarker, lastMeta } from '../runs.js';
  import { page } from '../nav.js';
  import RunMetaForm from './RunMetaForm.svelte';

  // Pseudo port served by the host's built-in simulator (internal/serial/sim.go).
  const SIMULATOR = 'SIMULATOR';

  let ports = [];
  let selectedPort = '';
  let baud = 115200;
  let loading = false;
  let error = '';

  async function loadPorts() {
    try {
      ports = await apiGet('/api/ports');
      if (ports.length > 0 && !selectedPort) selectedPort = ports[0];
    } catch (e) {
      error = e.message;
    }
  }

  async function connect() {
    loading = true; error = '';
    try {
      await apiPost('/api/connect', { port: selectedPort.trim(), baud });
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function disconnect() {
    loading = true; error = '';
    try {
      await apiPost('/api/disconnect');
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function startRecording() {
    loading = true; error = '';
    try {
      const name = `Run ${new Date().toLocaleString()}`;
      await apiPost('/api/runs', { ...lastMeta(), name });
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function stopRecording() {
    loading = true; error = '';
    try {
      await apiPost(`/api/runs/${$status.active_run_id}/stop`);
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  // ── markers on the run being recorded ─────────────────────────────────────
  let toast = '';
  let toastTimer;

  async function addLiveMarker() {
    if (!$status.recording) return;
    const label = `Marker ${$liveMarkers.length + 1}`;
    try {
      const m = await addMarker($status.active_run_id, label);
      liveMarkers.update(l => [...l, m]);
      toast = `⚑ ${label}`;
      clearTimeout(toastTimer);
      toastTimer = setTimeout(() => toast = '', 2000);
    } catch (e) {
      error = e.message;
    }
  }

  // M adds a marker while recording. Skipped in Runs, where M marks the run being viewed.
  function onKey(e) {
    if (!$status.recording || $page === 'runs') return;
    if (e.key !== 'm' && e.key !== 'M') return;
    if (e.ctrlKey || e.metaKey || e.altKey || e.target.closest?.('input, textarea, select, [contenteditable]')) return;
    addLiveMarker();
  }

  // ── metadata of the run being recorded ────────────────────────────────────
  let editRun = null;

  async function openRunInfo() {
    try {
      editRun = await apiGet(`/api/runs/${$status.active_run_id}`);
    } catch (e) {
      error = e.message;
    }
  }

  // Refresh port list whenever the WS connects
  $: if ($status.wsReady) loadPorts();
</script>

<svelte:window on:keydown={onKey} />

<header class="statusbar">
  <span class="brand">FSAE Telemetry</span>

  <div class="controls">
    {#if !$status.connected}
      <!-- Combobox: auto-suggests detected ports, but accepts any typed path -->
      <div class="port-input-wrap">
        <input
          type="text"
          class="port-input"
          bind:value={selectedPort}
          list="port-list"
          placeholder="Port (e.g. COM3, or SIMULATOR)"
          on:focus={loadPorts}
          spellcheck="false"
          autocomplete="off"
        />
        <datalist id="port-list">
          {#each ports as p}
            <option value={p}>{p === SIMULATOR ? 'Built-in simulator (no car needed)' : p}</option>
          {/each}
        </datalist>
        <button class="refresh-btn" on:click={loadPorts} title="Refresh port list">↻</button>
      </div>

      <input type="number" bind:value={baud} min="9600" step="9600" class="baud" />

      <button on:click={connect} disabled={loading || !selectedPort.trim()} class="btn-primary">
        Connect
      </button>
    {:else}
      {#if $status.port === SIMULATOR}
        <span class="port-label sim" title="Simulated data, not from the car">SIMULATOR</span>
      {:else}
        <span class="port-label">{$status.port}</span>
      {/if}

      {#if !$status.recording}
        <button on:click={startRecording} disabled={loading} class="btn-rec">
          ● Record
        </button>
      {:else}
        <button on:click={stopRecording} disabled={loading} class="btn-stop">
          ■ Stop
        </button>
        <span class="rec-indicator">REC</span>
        <button on:click={addLiveMarker} class="btn-marker" title="Add marker now (M)">⚑ Marker</button>
        <button on:click={openRunInfo} class="btn-ghost" title="Driver, mode, track, notes…">✎ Run info</button>
        {#if toast}<span class="toast">{toast}</span>{/if}
      {/if}

      <button on:click={disconnect} disabled={loading} class="btn-ghost">
        Disconnect
      </button>
    {/if}

    <span class="ws-dot" class:ready={$status.wsReady} title={$status.wsReady ? 'WS connected' : 'WS reconnecting…'}>●</span>
  </div>

  {#if error}
    <div class="error">{error}</div>
  {/if}
</header>

{#if editRun}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="overlay" on:click|self={() => editRun = null}>
    <div class="modal" role="dialog" aria-modal="true">
      <div class="modal-header">
        <h3>Run info</h3>
        <button class="close" on:click={() => editRun = null}>✕</button>
      </div>
      <RunMetaForm run={editRun} on:saved={() => editRun = null} />
    </div>
  </div>
{/if}

<style>
  .statusbar {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 8px 16px;
    background: #1a1a2e;
    border-bottom: 1px solid #2a2a4a;
    flex-wrap: wrap;
  }
  .brand {
    font-weight: 700;
    font-size: 1rem;
    color: #4FC3F7;
    margin-right: 8px;
  }
  .controls {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }

  .port-input-wrap {
    display: flex;
    align-items: center;
    background: #252540;
    border: 1px solid #3a3a5a;
    border-radius: 4px;
    overflow: hidden;
  }
  .port-input {
    background: transparent;
    border: none;
    color: #e0e0e0;
    padding: 4px 8px;
    font-size: 0.85rem;
    width: 200px;
    outline: none;
    font-family: monospace;
  }
  .port-input::placeholder { color: #555; font-family: system-ui, sans-serif; }
  .refresh-btn {
    background: none;
    border: none;
    border-left: 1px solid #3a3a5a;
    color: #666;
    padding: 4px 7px;
    cursor: pointer;
    font-size: 0.85rem;
    line-height: 1;
  }
  .refresh-btn:hover { color: #aaa; }

  input.baud {
    background: #252540;
    border: 1px solid #3a3a5a;
    color: #e0e0e0;
    padding: 4px 8px;
    border-radius: 4px;
    font-size: 0.85rem;
    width: 90px;
  }
  button {
    padding: 4px 12px;
    border-radius: 4px;
    border: none;
    cursor: pointer;
    font-size: 0.85rem;
    font-weight: 600;
  }
  button:disabled { opacity: 0.5; cursor: default; }
  .btn-primary { background: #4FC3F7; color: #000; }
  .btn-ghost { background: #2a2a4a; color: #aaa; border: 1px solid #3a3a5a; }
  .btn-rec { background: #e53935; color: #fff; }
  .btn-stop { background: #555; color: #fff; }
  .port-label { color: #81C784; font-size: 0.85rem; font-family: monospace; }
  .port-label.sim {
    color: #CE93D8;
    border: 1px solid #6a4a7a;
    border-radius: 4px;
    padding: 1px 6px;
  }
  .rec-indicator {
    color: #e53935;
    font-weight: 700;
    font-size: 0.75rem;
    animation: blink 1s step-end infinite;
  }
  @keyframes blink { 0%,100%{opacity:1} 50%{opacity:0} }
  .ws-dot { color: #555; font-size: 0.7rem; margin-left: 4px; }
  .ws-dot.ready { color: #81C784; }
  .error { width: 100%; color: #e57373; font-size: 0.8rem; }
  .btn-marker { background: #2e2810; color: #FFD54F; border: 1px solid #6b5a1a; }
  .btn-marker:hover { border-color: #FFD54F; }
  .toast { color: #FFD54F; font-size: 0.78rem; }

  .overlay {
    position: fixed; inset: 0; background: rgba(0,0,0,0.7);
    display: flex; align-items: center; justify-content: center; z-index: 100;
  }
  .modal {
    background: #1a1a38; border: 1px solid #3a3a5a; border-radius: 8px;
    width: 420px; max-width: 95vw; max-height: 90vh; overflow-y: auto; padding: 0 16px 16px;
  }
  .modal-header { display: flex; justify-content: space-between; align-items: center; padding: 12px 0; }
  .modal-header h3 { margin: 0; font-size: 1rem; color: #e0e0e0; }
  .close { background: none; border: none; color: #666; font-size: 1rem; cursor: pointer; padding: 0; }
  .close:hover { color: #e57373; }
</style>
