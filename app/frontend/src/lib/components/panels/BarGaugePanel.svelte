<script>
  import { onDestroy, getContext } from 'svelte';
  import { liveValues } from '../../ws.js';
  import { CHANNELS } from '../../channels.js';

  export let channels = [];
  export let min = -150;
  export let max = 150;

  const ctx = getContext('datasource');
  const valStore = ctx?.mode === 'playback' ? ctx.valuesStore : liveValues;

  let values = {};
  const unsub = valStore.subscribe(v => { values = v; });
  onDestroy(unsub);

  function pct(v) {
    return Math.max(0, Math.min(100, ((v - min) / (max - min)) * 100));
  }
</script>

<div class="bar-wrap">
  {#each channels as key}
    {@const meta  = CHANNELS[key] ?? {}}
    {@const v     = values[key] ?? 0}
    {@const p     = pct(v)}
    {@const warn  = p > 85 || p < 15}
    <div class="bar-row">
      <div class="bar-label">{meta.name ?? key}</div>
      <div class="bar-track">
        <div
          class="bar-fill"
          style="width: {p}%; background: {warn ? '#E57373' : (meta.color ?? '#4FC3F7')}"
        ></div>
      </div>
      <div class="bar-val" style="color: {meta.color ?? '#4FC3F7'}">{v.toFixed(1)}<span class="bar-unit">{meta.unit ?? ''}</span></div>
    </div>
  {/each}
</div>

<style>
  .bar-wrap {
    display: flex;
    flex-direction: column;
    justify-content: center;
    gap: 10px;
    padding: 8px 12px;
    height: 100%;
  }
  .bar-row {
    display: grid;
    grid-template-columns: 110px 1fr 60px;
    align-items: center;
    gap: 8px;
  }
  .bar-label {
    font-size: 0.72rem;
    color: #888;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .bar-track {
    height: 10px;
    background: #1e1e38;
    border-radius: 5px;
    overflow: hidden;
  }
  .bar-fill {
    height: 100%;
    border-radius: 5px;
    transition: width 80ms ease-out;
  }
  .bar-val {
    font-family: monospace;
    font-size: 0.8rem;
    text-align: right;
    font-weight: 600;
  }
  .bar-unit { color: #555; font-size: 0.65rem; margin-left: 1px; }
</style>
