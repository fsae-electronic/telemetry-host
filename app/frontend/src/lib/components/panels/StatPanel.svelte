<script>
  import { onDestroy, getContext } from 'svelte';
  import { liveValues } from '../../ws.js';
  import { CHANNELS } from '../../channels.js';

  export let channels = [];

  const ctx = getContext('datasource');
  const valStore = ctx?.mode === 'playback' ? ctx.valuesStore : liveValues;

  let values = {};
  const unsub = valStore.subscribe(v => { values = v; });
  onDestroy(unsub);

  const fmt = v => Number.isInteger(v) ? String(v) : v.toFixed(2);
</script>

<div class="stat-wrap">
  {#each channels as key}
    {@const meta = CHANNELS[key] ?? {}}
    <div class="stat-item" style="--color: {meta.color ?? '#4FC3F7'}">
      <div class="stat-label">{meta.name ?? key}</div>
      <div class="stat-value">{fmt(values[key] ?? 0)}</div>
      <div class="stat-unit">{meta.unit ?? ''}</div>
    </div>
  {/each}
</div>

<style>
  .stat-wrap {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
    align-items: center;
    justify-content: center;
    height: 100%;
    padding: 8px;
  }
  .stat-item {
    text-align: center;
    min-width: 80px;
  }
  .stat-label {
    font-size: 0.7rem;
    color: #666;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    margin-bottom: 4px;
  }
  .stat-value {
    font-size: 2rem;
    font-weight: 700;
    font-family: monospace;
    color: var(--color);
    line-height: 1;
  }
  .stat-unit {
    font-size: 0.75rem;
    color: #555;
    margin-top: 2px;
  }
</style>
