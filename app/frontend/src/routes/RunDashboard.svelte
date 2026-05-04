<script>
  import { setContext, onDestroy } from 'svelte';
  import { writable } from 'svelte/store';
  import { apiGet } from '../lib/ws.js';
  import { valuesAtIndex } from '../lib/playback.js';
  import { ALL_CHANNEL_KEYS } from '../lib/channels.js';
  import { createRuleEvaluator } from '../lib/ruleEngine.js';
  import DashboardGrid from '../lib/components/DashboardGrid.svelte';

  export let run;
  export let onBack;

  // ── context stores ──────────────────────────────────────────────────────────
  const samplesStore    = writable(null);
  const valuesStore     = writable({});
  const cursorTs        = writable(null);   // shared timestamp for all panels
  const faultEventsStore = writable([]);    // { ts: seconds, rule } computed after load

  // ── binary search: find index of nearest timestamp ──────────────────────────
  function nearestIdx(arr, target) {
    if (target <= arr[0])                 return 0;
    if (target >= arr[arr.length - 1])    return arr.length - 1;
    let lo = 0, hi = arr.length - 1;
    while (lo < hi - 1) {
      const mid = (lo + hi) >> 1;
      arr[mid] <= target ? (lo = mid) : (hi = mid);
    }
    return (target - arr[lo] <= arr[hi] - target) ? lo : hi;
  }

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
    faultEventsStore,
  });

  // Pre-compute all rule violations across the run's samples.
  // Runs a fresh evaluator (no shared live state) in one forward pass.
  function computePlaybackFaults(samples) {
    const evaluator = createRuleEvaluator();
    const events = [];
    const ts = samples[0];
    for (let i = 0; i < ts.length; i++) {
      const values = {};
      for (let j = 0; j < ALL_CHANNEL_KEYS.length; j++) {
        const v = samples[j + 1][i];
        if (v != null) values[ALL_CHANNEL_KEYS[j]] = v;
      }
      const { newFires } = evaluator.evaluate(values, ts[i] * 1000);
      for (const f of newFires) events.push({ ts: ts[i], rule: f.rule });
    }
    return events;
  }

  // ── load ────────────────────────────────────────────────────────────────────
  let loading = true;
  let error   = '';
  let totalSamples = 0;

  async function load() {
    loading = true; error = '';
    try {
      const raw = await apiGet(`/api/runs/${run.id}/samples`);
      if (!raw?.length) { error = 'No samples in this run.'; return; }
      const ts      = raw.map(r => r.ts);
      const ordered = ALL_CHANNEL_KEYS.map(k => raw.map(r => r[k] ?? null));
      totalSamples  = ts.length;
      const allSamples = [ts, ...ordered];
      samplesStore.set(allSamples);
      faultEventsStore.set(computePlaybackFaults(allSamples));
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

  // Keep valuesStore in sync when samples first arrive
  const unsubSamples = samplesStore.subscribe(s => {
    if (s) { valuesStore.set(valuesAtIndex(s, scrubIdx)); cursorTs.set(s[0][scrubIdx]); }
  });
  onDestroy(unsubSamples);

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

  // ── helpers ─────────────────────────────────────────────────────────────────
  function fmtTs(samples, idx) {
    const ts = samples?.[0]?.[idx];
    if (!ts) return '';
    const d = new Date(ts * 1000);
    return `${d.getHours().toString().padStart(2,'0')}:${d.getMinutes().toString().padStart(2,'0')}:${d.getSeconds().toString().padStart(2,'0')}.${Math.floor(d.getMilliseconds()/100)}`;
  }

  function duration(run) {
    if (!run.ended_at) return '';
    const s = Math.round(run.ended_at - run.started_at);
    return `${Math.floor(s / 60)}m ${s % 60}s`;
  }

  load();
</script>

<div class="run-dash">

  <!-- header ────────────────────────────────────────────────────────────────── -->
  <div class="run-header">
    <button class="back-btn" on:click={onBack}>← Runs</button>
    <div class="run-info">
      <span class="run-name">{run.name}</span>
      <span class="run-meta">{duration(run)}{totalSamples ? ` · ${totalSamples} samples` : ''}</span>
    </div>
  </div>

  <!-- grid ──────────────────────────────────────────────────────────────────── -->
  {#if loading}
    <div class="state-msg">Loading samples…</div>
  {:else if error}
    <div class="state-msg err">{error}</div>
  {:else}
    <div class="grid-scroll">
      <DashboardGrid editMode={false} />
    </div>

    <!-- transport bar ───────────────────────────────────────────────────────── -->
    <div class="transport">

      <!-- playback controls -->
      <div class="controls">
        <button class="ctrl-btn" on:click={restart} title="Restart">⏮</button>
        <button class="ctrl-btn play" on:click={togglePlay} title={playing ? 'Pause' : 'Play'}>
          {playing ? '⏸' : '▶'}
        </button>
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
      <span class="ts-edge">{fmtTs(samples, 0)}</span>
      <input
        class="scrub-track"
        type="range"
        min="0"
        max={maxIdx}
        step="1"
        bind:value={scrubIdx}
        on:input={() => { stopPlay(); scrubTo(scrubIdx); }}
      />
      <span class="ts-edge">{fmtTs(samples, maxIdx)}</span>

      <!-- current timestamp -->
      <span class="ts-now" class:playing>{fmtTs(samples, scrubIdx)}</span>

    </div>
  {/if}

</div>

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
  .run-info { display: flex; flex-direction: column; gap: 1px; }
  .run-name { font-size: 0.88rem; color: #e0e0e0; font-weight: 600; }
  .run-meta { font-size: 0.72rem; color: #555; }

  /* grid */
  .grid-scroll { flex: 1; overflow-y: auto; min-height: 0; }

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

  .scrub-track {
    flex: 1;
    accent-color: #4FC3F7;
    cursor: pointer;
    height: 4px;
  }
  .ts-edge {
    font-size: 0.65rem;
    color: #444;
    font-family: monospace;
    white-space: nowrap;
  }
  .ts-now {
    font-size: 0.72rem;
    color: #666;
    font-family: monospace;
    white-space: nowrap;
    min-width: 82px;
    text-align: right;
    transition: color 0.1s;
  }
  .ts-now.playing { color: #81C784; }

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
