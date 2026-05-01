<script>
  import { onDestroy } from 'svelte';
  import { liveValues } from '../../ws.js';
  import { CHANNELS } from '../../channels.js';

  export let channels = [];

  // Show one big number per channel
  let values = {};
  const unsub = liveValues.subscribe(v => { values = v; });
  onDestroy(unsub);
</script>

<div class="stat-wrap">
  {#each channels as key}
    {@const meta = CHANNELS[key] ?? {}}
    <div class="stat-item" style="--color: {meta.color ?? '#4FC3F7'}">
      <div class="stat-label">{meta.name ?? key}</div>
      <div class="stat-value">{(values[key] ?? 0).toFixed(2)}</div>
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
