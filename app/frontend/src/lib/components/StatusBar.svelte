<script>
  import { status, apiPost, apiGet } from '../ws.js';

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
      await apiPost('/api/runs', { name });
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

  // Refresh port list whenever the WS connects
  $: if ($status.wsReady) loadPorts();
</script>

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
          placeholder="Port (e.g. /dev/ttyUSB0)"
          on:focus={loadPorts}
          spellcheck="false"
          autocomplete="off"
        />
        <datalist id="port-list">
          {#each ports as p}
            <option value={p}>{p}</option>
          {/each}
        </datalist>
        <button class="refresh-btn" on:click={loadPorts} title="Refresh port list">↻</button>
      </div>

      <input type="number" bind:value={baud} min="9600" step="9600" class="baud" />

      <button on:click={connect} disabled={loading || !selectedPort.trim()} class="btn-primary">
        Connect
      </button>
    {:else}
      <span class="port-label">{$status.port}</span>

      {#if !$status.recording}
        <button on:click={startRecording} disabled={loading} class="btn-rec">
          ● Record
        </button>
      {:else}
        <button on:click={stopRecording} disabled={loading} class="btn-stop">
          ■ Stop
        </button>
        <span class="rec-indicator">REC</span>
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
</style>
