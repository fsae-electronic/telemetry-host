<script>
  import { liveValues } from '../../ws.js';
  import { CHANNELS } from '../../channels.js';

  export let channels = [];   // single channel expected

  const key   = channels[0];
  const meta  = CHANNELS[key] ?? {};
  const color = meta.color ?? '#4FC3F7';
  const unit  = meta.unit  ?? '';

  // Configurable range — could be per-panel later
  export let min = -150;
  export let max = 150;

  let value = 0;
  const unsub = liveValues.subscribe(v => { value = v[key] ?? 0; });
  import { onDestroy } from 'svelte';
  onDestroy(unsub);

  const SIZE   = 160;
  const CX     = SIZE / 2;
  const CY     = SIZE / 2;
  const R      = 64;
  const STROKE = 10;

  // Arc from 210° to 330° (240° sweep, starting at bottom-left)
  const START_DEG = 210;
  const SWEEP_DEG = 300;  // more arc feels better for a gauge

  function polarToXY(deg, r) {
    const rad = (deg - 90) * Math.PI / 180;
    return [CX + r * Math.cos(rad), CY + r * Math.sin(rad)];
  }

  function describeArc(startDeg, endDeg, r) {
    const [sx, sy] = polarToXY(startDeg, r);
    const [ex, ey] = polarToXY(endDeg, r);
    const large = (endDeg - startDeg + 360) % 360 > 180 ? 1 : 0;
    return `M ${sx} ${sy} A ${r} ${r} 0 ${large} 1 ${ex} ${ey}`;
  }

  $: clamped  = Math.max(min, Math.min(max, value));
  $: fraction = (clamped - min) / (max - min);
  $: endDeg   = START_DEG + fraction * SWEEP_DEG;
  $: bgPath   = describeArc(START_DEG, START_DEG + SWEEP_DEG, R);
  $: fgPath   = fraction > 0.001 ? describeArc(START_DEG, endDeg, R) : '';

  // Needle
  $: needleAngle = START_DEG + fraction * SWEEP_DEG - 90;
  $: [nx, ny] = polarToXY(needleAngle + 90, R - STROKE - 2);

  // Color shifts red when near limits
  $: gaugeColor = fraction > 0.85 || fraction < 0.15 ? '#E57373' : color;
</script>

<div class="gauge-wrap">
  <svg viewBox="0 0 {SIZE} {SIZE}" width={SIZE} height={SIZE}>
    <!-- Background track -->
    <path d={bgPath} fill="none" stroke="#1e1e38" stroke-width={STROKE} stroke-linecap="round"/>
    <!-- Value arc -->
    {#if fgPath}
      <path d={fgPath} fill="none" stroke={gaugeColor} stroke-width={STROKE} stroke-linecap="round"/>
    {/if}
    <!-- Needle dot -->
    <circle cx={nx} cy={ny} r="4" fill={gaugeColor}/>
    <!-- Center value -->
    <text x={CX} y={CY - 4} text-anchor="middle" fill="#e0e0e0" font-size="18" font-weight="700" font-family="monospace">
      {value.toFixed(1)}
    </text>
    <text x={CX} y={CY + 14} text-anchor="middle" fill="#666" font-size="11">
      {unit}
    </text>
    <!-- Min/max labels -->
    <text x="16" y={SIZE - 8} text-anchor="middle" fill="#444" font-size="9">{min}</text>
    <text x={SIZE - 16} y={SIZE - 8} text-anchor="middle" fill="#444" font-size="9">{max}</text>
  </svg>
</div>

<style>
  .gauge-wrap {
    display: flex;
    justify-content: center;
    align-items: center;
    height: 100%;
  }
</style>
