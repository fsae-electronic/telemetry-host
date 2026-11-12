<script>
  // One channel of several runs overlaid on a shared relative time axis.
  // All CompareCharts sharing rangeStore zoom together, and syncKey links
  // their cursors.
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import { get } from 'svelte/store';
  import uPlot from 'uplot';
  import 'uplot/dist/uPlot.min.css';
  import { fmtRel } from '../runs.js';

  export let title = '';
  export let unit = '';
  export let xs = [];          // seconds relative to each run's alignment point
  export let series = [];      // [{label, color, values}] aligned with xs
  export let markers = [];     // [{x, color, label}]
  export let width = 600;
  export let height = 200;
  export let syncKey = 'compare';
  export let rangeStore = null;

  const dispatch = createEventDispatcher();

  let el;
  let chart;
  let zoomed = false;
  let userScaleChange = false;
  let applyingShared = false;

  function drawMarkers(u) {
    if (!markers.length) return;
    const { left, top, width: bw, height: bh } = u.bbox;
    const c = u.ctx;
    c.save();
    c.setLineDash([2, 3]);
    c.font = `${10 * devicePixelRatio}px system-ui, sans-serif`;
    c.textBaseline = 'top';
    for (const m of markers) {
      const xPx = u.valToPos(m.x, 'x', true);
      if (xPx < left || xPx > left + bw) continue;
      c.strokeStyle = m.color + 'aa';
      c.beginPath();
      c.moveTo(xPx, top);
      c.lineTo(xPx, top + bh);
      c.stroke();
      if (m.label) {
        c.fillStyle = m.color;
        c.fillText(m.label, xPx + 3, top + 2);
      }
    }
    c.restore();
  }

  function opts() {
    return {
      width,
      height,
      legend: { show: false },
      cursor: {
        drag: { x: true, y: false, uni: 50 },
        sync: { key: syncKey, setSeries: false },
      },
      scales: { x: { time: false } },
      axes: [
        {
          stroke: '#555',
          grid: { stroke: '#1e1e38' },
          ticks: { stroke: '#2a2a4a' },
          values: (u, vals) => vals.map(v => (v == null ? '' : fmtRel(v))),
        },
        { stroke: '#555', grid: { stroke: '#1e1e38' }, ticks: { stroke: '#2a2a4a' }, size: 50 },
      ],
      series: [
        {},
        ...series.map(s => ({ label: s.label, stroke: s.color, width: 1.5, spanGaps: true })),
      ],
      hooks: {
        setSelect: [u => {
          if (u.select.width > 0) {
            userScaleChange = true;
            requestAnimationFrame(() => { userScaleChange = false; });
          }
        }],
        setScale: [(u, key) => {
          if (key !== 'x' || !userScaleChange) return;
          userScaleChange = false;
          zoomed = true;
          rangeStore?.set({ min: u.scales.x.min, max: u.scales.x.max });
        }],
        setCursor: [u => {
          const i = u.cursor.idx;
          if (i != null) dispatch('cursor', u.data[0][i]);
        }],
        draw: [drawMarkers],
      },
    };
  }

  function data() {
    return [xs, ...series.map(s => s.values)];
  }

  function applyRange(r) {
    if (!chart) return;
    applyingShared = true;
    if (r) {
      zoomed = true;
      chart.setScale('x', r);
    } else {
      zoomed = false;
      if (xs.length) chart.setScale('x', { min: xs[0], max: xs[xs.length - 1] });
    }
    applyingShared = false;
  }

  onMount(() => {
    chart = new uPlot(opts(), data(), el);
    builtFor = key;
    const r = rangeStore && get(rangeStore);
    if (r) applyRange(r);
  });
  onDestroy(() => chart?.destroy());

  const unsubRange = rangeStore?.subscribe(r => {
    if (!chart || applyingShared) return;
    const { min, max } = chart.scales.x;
    if (r && Math.abs(min - r.min) < 1e-6 && Math.abs(max - r.max) < 1e-6) return;
    applyRange(r);
  });
  onDestroy(() => unsubRange?.());

  // Series set changed (runs added/removed): rebuild; otherwise just resize.
  let builtFor = '';
  $: key = series.map(s => s.label + s.color).join('|');
  $: if (chart && key !== builtFor) {
    builtFor = key;
    chart.destroy();
    chart = new uPlot(opts(), data(), el);
    applyRange(rangeStore && get(rangeStore));
  }
  $: if (chart && builtFor === key) {
    chart.setData([xs, ...series.map(s => s.values)]);
    applyRange(rangeStore && get(rangeStore));
  }
  $: if (chart) chart.setSize({ width, height });
  $: if (chart && markers) chart.redraw(false);
</script>

<div class="cmp-chart">
  <div class="cmp-head">
    <span class="cmp-title">{title}</span>
    {#if unit}<span class="cmp-unit">{unit}</span>{/if}
    {#if zoomed}
      <button class="reset-btn" on:click={() => rangeStore ? rangeStore.set(null) : applyRange(null)}>↺ Reset zoom</button>
    {/if}
  </div>
  <div bind:this={el}></div>
</div>

<style>
  .cmp-chart { background: #12122a; border: 1px solid #2a2a4a; border-radius: 6px; padding: 4px 6px 2px; }
  .cmp-head { display: flex; align-items: center; gap: 8px; height: 22px; }
  .cmp-title { font-size: 0.75rem; color: #aaa; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; }
  .cmp-unit { font-size: 0.7rem; color: #555; }
  .reset-btn {
    margin-left: auto;
    background: #1e1e38;
    border: 1px solid #3a3a5a;
    color: #4FC3F7;
    padding: 1px 8px;
    border-radius: 4px;
    font-size: 0.7rem;
    cursor: pointer;
  }
</style>
