<script>
  import { createEventDispatcher } from 'svelte';
  import { META_FIELDS, updateRun, saveLastMeta } from '../runs.js';

  export let run;               // run object from the API
  export let suggestions = {};  // field key → previously used values

  const dispatch = createEventDispatcher();

  let form = {
    name: run.name ?? '',
    notes: run.notes ?? '',
    ...Object.fromEntries(META_FIELDS.map(f => [f.key, run[f.key] ?? ''])),
  };
  let saving = false;
  let error = '';
  let savedAt = null;

  async function save() {
    saving = true; error = '';
    try {
      const updated = await updateRun(run.id, form);
      saveLastMeta(updated);
      savedAt = new Date();
      dispatch('saved', updated);
    } catch (e) {
      error = e.message;
    } finally {
      saving = false;
    }
  }

  function options(f) {
    return [...new Set([...(f.options ?? []), ...(suggestions[f.key] ?? [])])].filter(Boolean);
  }
</script>

<form class="meta-form" on:submit|preventDefault={save}>
  <label>
    <span>Name</span>
    <input type="text" bind:value={form.name} />
  </label>

  {#each META_FIELDS as f}
    <label>
      <span>{f.label}</span>
      <input type="text" bind:value={form[f.key]} list="meta-{f.key}" placeholder={f.placeholder ?? ''} />
      <datalist id="meta-{f.key}">
        {#each options(f) as o}<option value={o}></option>{/each}
      </datalist>
    </label>
  {/each}

  <label>
    <span>Notes</span>
    <textarea rows="4" bind:value={form.notes}></textarea>
  </label>

  <div class="actions">
    {#if error}<span class="err">{error}</span>{/if}
    {#if savedAt && !error}<span class="ok">Saved</span>{/if}
    <button type="submit" class="btn-primary" disabled={saving}>{saving ? 'Saving…' : 'Save'}</button>
  </div>
</form>

<style>
  .meta-form { display: flex; flex-direction: column; gap: 10px; }
  label { display: flex; flex-direction: column; gap: 3px; }
  label span { font-size: 0.68rem; color: #777; text-transform: uppercase; letter-spacing: 0.05em; }
  input, textarea {
    background: #0f0f1e;
    border: 1px solid #2a2a4a;
    color: #e0e0e0;
    padding: 5px 8px;
    border-radius: 4px;
    font-size: 0.82rem;
    font-family: inherit;
    resize: vertical;
  }
  input:focus, textarea:focus { outline: none; border-color: #4FC3F7; }
  .actions { display: flex; align-items: center; justify-content: flex-end; gap: 10px; }
  .btn-primary {
    background: #4FC3F7; color: #000; border: none; padding: 5px 14px;
    border-radius: 4px; cursor: pointer; font-weight: 600; font-size: 0.8rem;
  }
  .btn-primary:disabled { opacity: 0.5; cursor: default; }
  .err { color: #e57373; font-size: 0.75rem; }
  .ok  { color: #81C784; font-size: 0.75rem; }
</style>
