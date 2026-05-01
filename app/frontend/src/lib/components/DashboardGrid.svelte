<script>
  import { createEventDispatcher } from 'svelte';
  import { dashboard } from '../dashboard.js';
  import PanelWrapper from './PanelWrapper.svelte';

  export let editMode = false;

  const dispatch = createEventDispatcher();

  const COL_COUNT = 12;
  const ROW_H     = 80;   // px per row unit
  const GAP       = 8;

  let gridEl;
  let colWidth = 0;

  function measureCols() {
    if (!gridEl) return;
    colWidth = (gridEl.clientWidth - GAP * (COL_COUNT + 1)) / COL_COUNT;
  }

  // Convert px offset to grid col/row delta during drag
  let dragState = null;

  function onDragStart(e, panel) {
    if (!editMode) return;
    const rect = gridEl.getBoundingClientRect();
    dragState = {
      id:       panel.id,
      origCol:  panel.col,
      origRow:  panel.row,
      startX:   e.clientX,
      startY:   e.clientY,
      ghost:    { col: panel.col, row: panel.row },
    };
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', panel.id);
  }

  function onDragOver(e) {
    if (!dragState || !editMode) return;
    e.preventDefault();
    const dx = e.clientX - dragState.startX;
    const dy = e.clientY - dragState.startY;
    const dCol = Math.round(dx / (colWidth + GAP));
    const dRow = Math.round(dy / (ROW_H + GAP));
    dragState.ghost = {
      col: Math.max(1, dragState.origCol + dCol),
      row: Math.max(1, dragState.origRow + dRow),
    };
    dragState = dragState; // trigger reactivity
  }

  function onDrop(e) {
    if (!dragState || !editMode) return;
    e.preventDefault();
    dashboard.movePanel(dragState.id, dragState.ghost.col, dragState.ghost.row);
    dragState = null;
  }

  function onDragEnd() { dragState = null; }

  // Resize via mousedown on the handle in the corner
  let resizeState = null;

  function onResizeStart(e, panel) {
    e.preventDefault();
    e.stopPropagation();
    resizeState = {
      id:       panel.id,
      origCols: panel.cols,
      origRows: panel.rows,
      startX:   e.clientX,
      startY:   e.clientY,
    };
    window.addEventListener('mousemove', onResizeMove);
    window.addEventListener('mouseup',   onResizeEnd);
  }

  function onResizeMove(e) {
    if (!resizeState) return;
    measureCols();
    const dx = e.clientX - resizeState.startX;
    const dy = e.clientY - resizeState.startY;
    const dCols = Math.round(dx / (colWidth + GAP));
    const dRows = Math.round(dy / (ROW_H  + GAP));
    const newCols = Math.max(1, Math.min(COL_COUNT, resizeState.origCols + dCols));
    const newRows = Math.max(1, resizeState.origRows + dRows);
    dashboard.updatePanel(resizeState.id, { cols: newCols, rows: newRows });
  }

  function onResizeEnd() {
    resizeState = null;
    window.removeEventListener('mousemove', onResizeMove);
    window.removeEventListener('mouseup',   onResizeEnd);
  }

  function panelStyle(panel) {
    const p = (dragState?.id === panel.id) ? dragState.ghost : panel;
    return [
      `grid-column: ${p.col} / span ${panel.cols}`,
      `grid-row:    ${p.row} / span ${panel.rows}`,
    ].join(';');
  }

  import { onMount } from 'svelte';
  onMount(() => {
    measureCols();
    const ro = new ResizeObserver(measureCols);
    ro.observe(gridEl);
    return () => ro.disconnect();
  });
</script>

<div
  class="grid"
  style="--col-count:{COL_COUNT}; --row-h:{ROW_H}px; --gap:{GAP}px"
  bind:this={gridEl}
  on:dragover={onDragOver}
  on:drop={onDrop}
>
  {#each $dashboard as panel (panel.id)}
    <PanelWrapper
      {panel}
      {editMode}
      style={panelStyle(panel)}
      on:dragstart={(e) => onDragStart(e.detail, panel)}
      on:dragend={onDragEnd}
      on:resizestart={(e) => onResizeStart(e.detail, panel)}
      on:edit={() => dispatch('edit', panel)}
      on:remove={() => dashboard.removePanel(panel.id)}
    />
  {/each}
</div>

<style>
  .grid {
    display: grid;
    grid-template-columns: repeat(var(--col-count), 1fr);
    grid-auto-rows: var(--row-h);
    gap: var(--gap);
    padding: var(--gap);
    width: 100%;
    min-height: 100%;
    align-content: start;
  }
</style>
