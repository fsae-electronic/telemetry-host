<script>
  import { onDestroy } from 'svelte';
  import { liveBuffer, latestPacket, MAX_POINTS } from '../lib/ws.js';
  import UPlotChart from '../lib/components/UPlotChart.svelte';
  import { CHANNEL_COLORS, channelLabel, channelKey } from '../lib/colors.js';

  const NUM_CH = 12;

  // Which channels are shown on the main chart
  let visibleChannels = new Set(Array.from({ length: NUM_CH }, (_, i) => i));

  // Chart component ref for the main multi-channel chart
  let chartRef;

  // Series config for uPlot (one per channel)
  $: mainSeries = Array.from({ length: NUM_CH }, (_, i) => ({
    label: channelLabel(i),
    stroke: CHANNEL_COLORS[i],
    width: 1.5,
    show: visibleChannels.has(i),
  }));

  // Convert ring buffer to uPlot data arrays on each update
  // We do this in a reactive statement so it re-runs when liveBuffer updates
  let uplotData = [[], ...Array.from({ length: NUM_CH }, () => [])];

  const unsub = liveBuffer.subscribe(buf => {
    if (buf.count === 0) return;

    const n = buf.count;
    // When buffer is not yet full, oldest is at 0; once full, oldest is at head.
    const oldestIdx = n < MAX_POINTS ? 0 : buf.head;

    const ts = new Array(n);
    const ch = Array.from({ length: NUM_CH }, () => new Array(n));

    for (let j = 0; j < n; j++) {
      const idx = (oldestIdx + j) % MAX_POINTS;
      ts[j] = buf.ts[idx];
      for (let i = 0; i < NUM_CH; i++) {
        ch[i][j] = buf.channels[i][idx];
      }
    }

    uplotData = [ts, ...ch];
    chartRef?.setData(uplotData);
  });

  onDestroy(unsub);

  // Current values from latest packet
  let currentValues = Array(NUM_CH).fill(null);
  const unsubLatest = latestPacket.subscribe(p => {
    if (!p) return;
    for (let i = 0; i < NUM_CH; i++) {
      currentValues[i] = p.data[channelKey(i)] ?? null;
    }
    currentValues = currentValues;
  });
  onDestroy(unsubLatest);

  function toggleChannel(i) {
    if (visibleChannels.has(i)) {
      visibleChannels.delete(i);
    } else {
      visibleChannels.add(i);
    }
    visibleChannels = new Set(visibleChannels);
  }
</script>

<div class="live">
  <!-- Channel value cards -->
  <div class="cards">
    {#each Array.from({ length: NUM_CH }, (_, i) => i) as i}
      <button
        class="card"
        class:hidden={!visibleChannels.has(i)}
        style="--color: {CHANNEL_COLORS[i]}"
        on:click={() => toggleChannel(i)}
        title="Click to toggle on chart"
      >
        <span class="ch-label">{channelLabel(i)}</span>
        <span class="ch-value">
          {currentValues[i] !== null ? currentValues[i].toFixed(2) : '—'}
        </span>
        <span class="ch-dot">●</span>
      </button>
    {/each}
  </div>

  <!-- Main chart -->
  <div class="chart-wrap">
    <UPlotChart
      bind:this={chartRef}
      data={uplotData}
      series={mainSeries}
      width={900}
      height={340}
      live={true}
    />
  </div>
</div>

<style>
  .live {
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .cards {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
    gap: 8px;
  }
  .card {
    background: #1e1e38;
    border: 2px solid var(--color, #4FC3F7);
    border-radius: 6px;
    padding: 10px 12px;
    cursor: pointer;
    text-align: left;
    position: relative;
    transition: opacity 0.15s;
  }
  .card.hidden {
    opacity: 0.35;
    border-color: #333;
  }
  .ch-label {
    display: block;
    font-size: 0.7rem;
    color: #888;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .ch-value {
    display: block;
    font-size: 1.2rem;
    font-weight: 700;
    color: var(--color, #e0e0e0);
    font-family: monospace;
    margin-top: 2px;
  }
  .ch-dot {
    position: absolute;
    top: 6px;
    right: 8px;
    font-size: 0.55rem;
    color: var(--color);
  }
  .chart-wrap {
    background: #141428;
    border-radius: 8px;
    padding: 12px;
    overflow-x: auto;
  }
</style>
