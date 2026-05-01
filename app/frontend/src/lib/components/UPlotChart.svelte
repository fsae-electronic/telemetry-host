<script>
  import { onMount, onDestroy } from 'svelte';
  import uPlot from 'uplot';
  import 'uplot/dist/uPlot.min.css';

  export let data = [[], []];     // uPlot format: [timestamps, ...series]
  export let series = [];         // uPlot series config
  export let width = 600;
  export let height = 200;
  export let live = false;        // if true, x-axis autoscrolls

  let container;
  let chart;

  const opts = () => ({
    width,
    height,
    cursor: { show: !live },
    legend: { show: series.length <= 4 },
    scales: {
      x: { time: false },
      y: {},
    },
    axes: [
      {
        stroke: '#888',
        grid: { stroke: '#333' },
        ticks: { stroke: '#555' },
        values: (u, vals) => vals.map(v => {
          const d = new Date(v * 1000);
          return `${d.getMinutes().toString().padStart(2,'0')}:${d.getSeconds().toString().padStart(2,'0')}`;
        }),
      },
      {
        stroke: '#888',
        grid: { stroke: '#333' },
        ticks: { stroke: '#555' },
      },
    ],
    series: [
      {},
      ...series,
    ],
  });

  onMount(() => {
    chart = new uPlot(opts(), data, container);
  });

  onDestroy(() => {
    chart?.destroy();
  });

  export function setData(newData) {
    if (!chart) return;
    chart.setData(newData);
    if (live && newData[0].length > 0) {
      const last = newData[0][newData[0].length - 1];
      const windowSecs = 30;
      chart.setScale('x', { min: last - windowSecs, max: last });
    }
  }

  $: if (chart && (width || height)) {
    chart.setSize({ width, height });
  }
</script>

<div bind:this={container}></div>
