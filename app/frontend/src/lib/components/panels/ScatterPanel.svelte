<script>
  import { onMount, onDestroy } from 'svelte';
  import uPlot from 'uplot';
  import 'uplot/dist/uPlot.min.css';
  import { liveBuffer, MAX_POINTS } from '../../ws.js';
  import { CHANNELS } from '../../channels.js';

  export let channels = [];   // expects exactly 2 keys: [xKey, yKey]
  export let width = 300;
  export let height = 200;

  const [xKey, yKey] = channels;
  const xMeta = CHANNELS[xKey] ?? {};
  const yMeta = CHANNELS[yKey] ?? {};

  let container;
  let chart;

  const opts = () => ({
    width,
    height,
    cursor: { show: true },
    legend: { show: false },
    scales: { x: { time: false } },
    axes: [
      { label: xMeta.name ?? xKey, stroke: '#666', grid: { stroke: '#1e1e38' } },
      { label: yMeta.name ?? yKey, stroke: '#666', grid: { stroke: '#1e1e38' } },
    ],
    series: [
      {},
      {
        label: yMeta.name ?? yKey,
        stroke: yMeta.color ?? '#4FC3F7',
        fill: (yMeta.color ?? '#4FC3F7') + '22',
        width: 0,
        points: { show: true, size: 3, fill: yMeta.color ?? '#4FC3F7' },
      },
    ],
  });

  onMount(() => {
    chart = new uPlot(opts(), [[], []], container);
  });

  onDestroy(() => chart?.destroy());

  const unsub = liveBuffer.subscribe(buf => {
    if (!chart || buf.count === 0 || !xKey || !yKey) return;
    const n = buf.count;
    const oldestIdx = n < MAX_POINTS ? 0 : buf.head;
    const xs = new Array(n);
    const ys = new Array(n);
    for (let j = 0; j < n; j++) {
      const idx = (oldestIdx + j) % MAX_POINTS;
      xs[j] = buf.channels[xKey][idx];
      ys[j] = buf.channels[yKey][idx];
    }
    // Sort by x for scatter rendering
    const pairs = xs.map((x, i) => [x, ys[i]]).sort((a, b) => a[0] - b[0]);
    chart.setData([pairs.map(p => p[0]), pairs.map(p => p[1])]);
  });

  onDestroy(unsub);

  $: if (chart && (width || height)) chart.setSize({ width, height });
</script>

<div bind:this={container} class="scatter-wrap"></div>

<style>
  .scatter-wrap { width: 100%; height: 100%; }
</style>
