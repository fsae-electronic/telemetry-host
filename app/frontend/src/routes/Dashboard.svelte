<script>
  import DashboardGrid from '../lib/components/DashboardGrid.svelte';
  import PanelModal    from '../lib/components/PanelModal.svelte';
  import { dashboard } from '../lib/dashboard.js';

  let editMode   = false;
  let showModal  = false;
  let editPanel  = null;   // null = add, object = edit

  function openAdd() {
    editPanel = null;
    showModal = true;
  }

  function openEdit(e) {
    editPanel = e.detail;
    showModal = true;
  }

  function onSave(e) {
    const data = e.detail;
    if (editPanel) {
      dashboard.updatePanel(editPanel.id, data);
    } else {
      dashboard.addPanel({
        ...data,
        id:  `p-${Date.now()}`,
        col: 1,
        row: 99, // append below existing
      });
    }
    showModal = false;
  }
</script>

<div class="dash-toolbar">
  <button
    class="toolbar-btn"
    class:active={editMode}
    on:click={() => editMode = !editMode}
  >
    {editMode ? '✓ Done' : '⊹ Edit Layout'}
  </button>
  {#if editMode}
    <button class="toolbar-btn" on:click={openAdd}>＋ Add Panel</button>
    <button class="toolbar-btn warn" on:click={dashboard.reset}>↺ Reset</button>
  {/if}
</div>

<div class="dash-scroll">
  <DashboardGrid {editMode} on:edit={openEdit} />
</div>

{#if showModal}
  <PanelModal
    panel={editPanel}
    on:save={onSave}
    on:close={() => showModal = false}
  />
{/if}

<style>
  .dash-toolbar {
    display: flex;
    gap: 6px;
    padding: 6px 12px;
    background: #0f0f1e;
    border-bottom: 1px solid #1e1e38;
    align-items: center;
  }
  .toolbar-btn {
    background: #1a1a38;
    border: 1px solid #2a2a4a;
    color: #aaa;
    padding: 4px 12px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.8rem;
    font-weight: 600;
    transition: all 0.1s;
  }
  .toolbar-btn:hover { border-color: #4FC3F7; color: #4FC3F7; }
  .toolbar-btn.active { border-color: #81C784; color: #81C784; }
  .toolbar-btn.warn:hover { border-color: #FFB74D; color: #FFB74D; }

  .dash-scroll {
    overflow-y: auto;
    height: calc(100vh - 90px); /* statusbar + nav + toolbar */
  }
</style>
