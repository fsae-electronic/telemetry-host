<script>
  import { createEventDispatcher } from 'svelte';
  import TimeSeriesPanel from './panels/TimeSeriesPanel.svelte';
  import GaugePanel      from './panels/GaugePanel.svelte';
  import StatPanel       from './panels/StatPanel.svelte';
  import BarGaugePanel   from './panels/BarGaugePanel.svelte';
  import ScatterPanel    from './panels/ScatterPanel.svelte';

  export let panel;
  export let editMode = false;
  export let style = '';

  const dispatch = createEventDispatcher();

  const PANEL_MAP = {
    timeseries: TimeSeriesPanel,
    gauge:      GaugePanel,
    stat:       StatPanel,
    bargauge:   BarGaugePanel,
    scatter:    ScatterPanel,
  };

  let wrapEl;
  let innerW = 400;
  let innerH = 200;

  import { onMount } from 'svelte';
  onMount(() => {
    const ro = new ResizeObserver(entries => {
      for (const e of entries) {
        innerW = e.contentRect.width;
        innerH = e.contentRect.height - 32; // 32 = title bar height
      }
    });
    ro.observe(wrapEl);
    return () => ro.disconnect();
  });

  function onDragStart(e) {
    if (!editMode) return;
    dispatch('dragstart', e);
  }
</script>

<div
  class="panel"
  class:edit={editMode}
  {style}
  bind:this={wrapEl}
  draggable={editMode}
  on:dragstart={onDragStart}
  on:dragend={() => dispatch('dragend')}
>
  <!-- Title bar -->
  <div class="panel-header">
    <span class="panel-title" class:drag-handle={editMode}>{panel.title}</span>
    {#if editMode}
      <div class="panel-actions">
        <button title="Edit" on:click={() => dispatch('edit')}>✎</button>
        <button title="Remove" class="danger" on:click={() => dispatch('remove')}>✕</button>
      </div>
    {/if}
  </div>

  <!-- Panel content -->
  <div class="panel-body">
    {#if panel.channels?.length > 0}
      <svelte:component
        this={PANEL_MAP[panel.type] ?? TimeSeriesPanel}
        channels={panel.channels}
        width={innerW}
        height={Math.max(40, innerH)}
        min={panel.min}
        max={panel.max}
      />
    {:else}
      <div class="empty-panel">No channels configured</div>
    {/if}
  </div>

  <!-- Resize handle (edit mode only) -->
  {#if editMode}
    <div
      class="resize-handle"
      on:mousedown|preventDefault={(e) => dispatch('resizestart', e)}
    >⊿</div>
  {/if}
</div>

<style>
  .panel {
    background: #12122a;
    border: 1px solid #2a2a4a;
    border-radius: 6px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    position: relative;
    transition: border-color 0.15s;
    min-height: 0;
  }
  .panel.edit {
    border-color: #3a3a6a;
    cursor: grab;
  }
  .panel.edit:active { cursor: grabbing; }

  .panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 4px 10px;
    background: #1a1a38;
    border-bottom: 1px solid #2a2a4a;
    height: 32px;
    flex-shrink: 0;
  }
  .panel-title {
    font-size: 0.78rem;
    color: #aaa;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .drag-handle { cursor: grab; }

  .panel-actions {
    display: flex;
    gap: 4px;
  }
  .panel-actions button {
    background: none;
    border: 1px solid #3a3a5a;
    color: #888;
    font-size: 0.75rem;
    cursor: pointer;
    border-radius: 3px;
    padding: 1px 5px;
    line-height: 1.4;
  }
  .panel-actions button:hover { color: #ccc; border-color: #5a5a8a; }
  .panel-actions button.danger:hover { color: #e57373; border-color: #e57373; }

  .panel-body {
    flex: 1;
    overflow: hidden;
    min-height: 0;
  }

  .empty-panel {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
    color: #444;
    font-size: 0.8rem;
  }

  .resize-handle {
    position: absolute;
    bottom: 2px;
    right: 4px;
    color: #3a3a6a;
    font-size: 0.9rem;
    cursor: se-resize;
    user-select: none;
    line-height: 1;
  }
  .resize-handle:hover { color: #7a7aaa; }
</style>
