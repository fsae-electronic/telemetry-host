<script>
  import { onMount, onDestroy } from 'svelte';
  import uPlot from 'uplot';
  import 'uplot/dist/uPlot.min.css';
  import { liveBuffer, MAX_POINTS } from '../../ws.js';
  import { CHANNELS } from '../../channels.js';

  export let channels = [];   // array of channel keys
  export let width = 400;
  export let height = 200;
  export let live = true;
  export let data = null;     // static data for run playback

  let container;
  let chart;

  const series = channels.map(key => ({
    label: CHANNELS[key]?.name ?? key,
    stroke: CHANNELS[key]?.color ?? '#fff',
    width: 1.5,
  }));

  const opts = () => ({
    width,
    height,
    cursor: { show: true, drag: { x: true, y: false } },
    legend: { show: channels.length <= 6 },
    scales: { x: { time: false } },
    axes: [
      {
        stroke: '#666',
        grid: { stroke: '#1e1e38' },
        ticks: { stroke: '#333' },
        values: (u, vals) => vals.map(v => {
          if (v == null) return '';
          const d = new Date(v * 1000);
          return `${d.getMinutes().toString().padStart(2,'0')}:${d.getSeconds().toString().padStart(2,'0')}`;
        }),
      },
      {
        stroke: '#666',
        grid: { stroke: '#1e1e38' },
        ticks: { stroke: '#333' },
      },
    ],
    series: [{}, ...series],
  });

  onMount(() => {
    const initData = data ?? [[], ...channels.map(() => [])];
    chart = new uPlot(opts(), initData, container);
  });

  onDestroy(() => chart?.destroy());

  // Live mode: subscribe to ring buffer
  let unsub;
  if (live) {
    unsub = liveBuffer.subscribe(buf => {
      if (!chart || buf.count === 0) return;
      const n = buf.count;
      const oldestIdx = n < MAX_POINTS ? 0 : buf.head;
      const ts = new Array(n);
      const ch = channels.map(() => new Array(n));
      for (let j = 0; j < n; j++) {
        const idx = (oldestIdx + j) % MAX_POINTS;
        ts[j] = buf.ts[idx];
        for (let i = 0; i < channels.length; i++) {
          ch[i][j] = buf.channels[channels[i]][idx];
        }
      }
      const udata = [ts, ...ch];
      chart.setData(udata);
      const last = ts[ts.length - 1];
      chart.setScale('x', { min: last - 30, max: last });
    });
  }

  onDestroy(() => unsub?.());

  // Static playback mode
  $: if (!live && data && chart) {
    chart.setData(data);
  }

  $: if (chart && (width || height)) {
    chart.setSize({ width, height });
  }
</script>

<div bind:this={container} class="ts-wrap"></div>

<style>
  .ts-wrap { width: 100%; height: 100%; }
  :global(.ts-wrap .uplot) { background: transparent !important; }
</style>
