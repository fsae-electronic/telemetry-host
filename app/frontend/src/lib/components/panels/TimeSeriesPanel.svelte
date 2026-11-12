<script>
  import { onMount, onDestroy, tick, getContext } from 'svelte';
  import { get } from 'svelte/store';
  import uPlot from 'uplot';
  import 'uplot/dist/uPlot.min.css';
  import { liveBuffer, liveMarkers, MAX_POINTS } from '../../ws.js';
  import { CHANNELS } from '../../channels.js';

  export let channels = [];
  export let width  = 400;
  export let height = 200;
  export let live   = true;
  export let data   = null;   // uPlot-format static data for run playback

  // ─── playback context (present only inside RunDashboard) ───────────────────
  const dsCtx      = getContext('datasource');
  const isPlayback = dsCtx?.mode === 'playback';

  // Declared first: the store subscriptions below run immediately and check it.
  let mainEl;
  let mainChart;

  // Mirror of the shared cursor timestamp — updated via subscription below.
  let sharedCursorTs    = null;
  let cursorRafPending  = false;

  if (isPlayback && dsCtx?.cursorTs) {
    const unsubCursor = dsCtx.cursorTs.subscribe(ts => {
      sharedCursorTs = ts;
      if (!cursorRafPending) {
        cursorRafPending = true;
        requestAnimationFrame(() => { mainChart?.redraw(false); cursorRafPending = false; });
      }
    });
    onDestroy(unsubCursor);
  }

  // Vertical dashed line drawn at the shared cursor position across all panels.
  function drawSharedCursor(u) {
    if (!isPlayback || sharedCursorTs == null) return;
    const xPx = u.valToPos(sharedCursorTs, 'x', true); // canvas pixels
    const { left, top, width: bw, height: bh } = u.bbox;
    if (xPx < left || xPx > left + bw) return;
    const c = u.ctx;
    c.save();
    c.strokeStyle = 'rgba(255,255,255,0.55)';
    c.lineWidth   = 1;
    c.setLineDash([3, 3]);
    c.beginPath();
    c.moveTo(xPx, top);
    c.lineTo(xPx, top + bh);
    c.stroke();
    c.restore();
  }

  // ─── markers ───────────────────────────────────────────────────────────────
  // Run markers in playback, markers of the active recording in live mode.
  let markers = [];
  const unsubMarkers = (isPlayback ? dsCtx.markersStore : liveMarkers)?.subscribe(m => {
    markers = m ?? [];
    mainChart?.redraw(false);
  });
  onDestroy(() => unsubMarkers?.());

  function drawMarkers(u) {
    if (!markers.length) return;
    const { left, top, width: bw, height: bh } = u.bbox;
    const c = u.ctx;
    c.save();
    c.font = `${10 * devicePixelRatio}px system-ui, sans-serif`;
    c.textBaseline = 'top';
    for (const m of markers) {
      const xPx = u.valToPos(m.ts, 'x', true);
      if (xPx < left || xPx > left + bw) continue;
      c.strokeStyle = 'rgba(255,213,79,0.8)';
      c.lineWidth = 1;
      c.beginPath();
      c.moveTo(xPx, top);
      c.lineTo(xPx, top + bh);
      c.stroke();
      if (m.label) {
        c.fillStyle = 'rgba(255,213,79,0.95)';
        c.fillText(m.label, xPx + 3, top + 2);
      }
    }
    c.restore();
  }

  // ─── layout ────────────────────────────────────────────────────────────────
  const MINIMAP_H  = 52;
  const LEG_ROW_H  = 26;
  const LEG_HEAD_H = 24;

  let legendOpen = true;
  $: legH    = legendOpen ? LEG_HEAD_H + channels.length * LEG_ROW_H : LEG_HEAD_H;
  $: chartH  = Math.max(60, height - legH - (userZoomed ? MINIMAP_H + 2 : 0));

  // ─── per-channel state ──────────────────────────────────────────────────────
  // visible[key]          → channel shown on chart
  // trackerOn[key][stat]  → dashed line drawn at that stat value
  let visible    = Object.fromEntries(channels.map(k => [k, true]));
  let trackerOn  = Object.fromEntries(channels.map(k => [k, { min: false, max: false, avg: false }]));

  // ─── stats ──────────────────────────────────────────────────────────────────
  let stats = {};   // key → { min, max, avg } | null

  function computeStats(udata) {
    if (!udata?.[0]?.length) { stats = {}; return; }
    const xs = udata[0];
    const lo = userZoomed ? mainChart?.scales?.x?.min : null;
    const hi = userZoomed ? mainChart?.scales?.x?.max : null;
    const next = {};
    channels.forEach((key, i) => {
      const ys = udata[i + 1];
      if (!ys) return;
      let mn = Infinity, mx = -Infinity, sum = 0, n = 0;
      for (let j = 0; j < ys.length; j++) {
        if (lo != null && (xs[j] < lo || xs[j] > hi)) continue;
        const v = ys[j];
        if (v == null) continue;
        if (v < mn) mn = v;
        if (v > mx) mx = v;
        sum += v; n++;
      }
      next[key] = n > 0 ? { min: mn, max: mx, avg: sum / n } : null;
    });
    stats = next;
  }

  function hasTrackers() {
    return channels.some(k => trackerOn[k]?.min || trackerOn[k]?.max || trackerOn[k]?.avg);
  }

  // ─── uPlot hook: draw tracker lines ────────────────────────────────────────
  function drawTrackers(u) {
    if (!hasTrackers()) return;
    const ctx = u.ctx;
    const { left, top, width: bw, height: bh } = u.bbox;
    ctx.save();
    channels.forEach((key, i) => {
      const s = stats[key];
      const t = trackerOn[key];
      if (!s || !t) return;
      const col = CHANNELS[key]?.color ?? '#aaa';
      for (const stat of ['min', 'max', 'avg']) {
        if (!t[stat]) continue;
        const yPx = u.valToPos(s[stat], 'y', true);
        if (yPx < top || yPx > top + bh) continue;
        ctx.strokeStyle = col + 'bb';
        ctx.lineWidth   = 1;
        ctx.setLineDash([4, 4]);
        ctx.beginPath();
        ctx.moveTo(left, yPx);
        ctx.lineTo(left + bw, yPx);
        ctx.stroke();
        ctx.setLineDash([]);
        ctx.fillStyle    = col + 'ee';
        ctx.font         = '10px monospace';
        ctx.textBaseline = 'bottom';
        ctx.fillText(`${stat} ${s[stat].toFixed(2)}`, left + 4, yPx - 1);
      }
    });
    ctx.restore();
  }

  // ─── zoom state ─────────────────────────────────────────────────────────────
  // We distinguish user-initiated zoom from the live auto-scroll.
  let userZoomed = false;

  // In playback every panel shares one x range (dsCtx.xRange): a zoom or pan
  // made in one panel is published there and applied by all the others.
  const sharedRange = isPlayback ? dsCtx.xRange : null;
  let userScaleChange = false;   // the next x setScale comes from this panel's user
  let applyingShared  = false;   // setScale call made to follow sharedRange

  function publishRange() {
    if (!sharedRange || applyingShared || !mainChart) return;
    sharedRange.set({ min: mainChart.scales.x.min, max: mainChart.scales.x.max });
  }

  const unsubRange = sharedRange?.subscribe(r => {
    if (!mainChart) return;
    if (r == null) {
      if (userZoomed) applyReset();
      return;
    }
    const { min, max } = mainChart.scales.x;
    if (Math.abs(min - r.min) < 1e-6 && Math.abs(max - r.max) < 1e-6) return;
    applyingShared = true;
    userZoomed = true;
    mainChart.setScale('x', r);
    applyingShared = false;
  });
  onDestroy(() => unsubRange?.());

  function resetZoom() {
    if (sharedRange) sharedRange.set(null);
    else applyReset();
  }

  function applyReset() {
    userZoomed = false;
    if (!live && currentData?.[0]?.length) {
      const xs = currentData[0];
      mainChart?.setScale('x', { min: xs[0], max: xs[xs.length - 1] });
    }
    // live mode: next buffer tick restores the rolling window automatically
    tick().then(drawMinimap);
  }

  // ─── minimap canvas ─────────────────────────────────────────────────────────
  let minimapEl;   // bound when userZoomed

  async function drawMinimap() {
    await tick();  // ensure canvas is in DOM when called after reactive update
    if (!minimapEl || !currentData?.[0]?.length) return;
    const ctx = minimapEl.getContext('2d');
    const W = minimapEl.width;
    const H = minimapEl.height;
    ctx.clearRect(0, 0, W, H);
    ctx.fillStyle = '#0d0d20';
    ctx.fillRect(0, 0, W, H);

    const xs = currentData[0];
    const xMin = xs[0], xMax = xs[xs.length - 1];
    const xRange = xMax - xMin;
    if (xRange === 0) return;

    // y range across visible channels
    let yMin = Infinity, yMax = -Infinity;
    channels.forEach((key, ci) => {
      if (!visible[key]) return;
      for (const v of currentData[ci + 1]) { if (v < yMin) yMin = v; if (v > yMax) yMax = v; }
    });
    if (!isFinite(yMin)) return;
    const yRange = yMax - yMin || 1;
    const PAD = 3;
    const px = x => (x - xMin) / xRange * W;
    const py = y => PAD + (1 - (y - yMin) / yRange) * (H - PAD * 2);

    // dim outside viewport
    const vl = mainChart ? px(mainChart.scales.x.min) : 0;
    const vr = mainChart ? px(mainChart.scales.x.max) : W;
    ctx.fillStyle = 'rgba(0,0,0,0.5)';
    ctx.fillRect(0, 0, vl, H);
    ctx.fillRect(vr, 0, W - vr, H);

    // channel lines
    channels.forEach((key, ci) => {
      if (!visible[key]) return;
      const ys = currentData[ci + 1];
      ctx.strokeStyle = (CHANNELS[key]?.color ?? '#fff') + '99';
      ctx.lineWidth = 1;
      ctx.beginPath();
      let first = true;
      for (let j = 0; j < xs.length; j++) {
        const x = px(xs[j]), y = py(ys[j]);
        if (first) { ctx.moveTo(x, y); first = false; } else ctx.lineTo(x, y);
      }
      ctx.stroke();
    });

    // viewport box
    ctx.strokeStyle = 'rgba(79,195,247,0.7)';
    ctx.lineWidth = 1;
    ctx.strokeRect(vl, 0, vr - vl, H);
    ctx.fillStyle = 'rgba(79,195,247,0.07)';
    ctx.fillRect(vl, 0, vr - vl, H);
  }

  // Drag/click on minimap to pan
  function onMinimapPointer(e) {
    const xs = currentData?.[0];
    if (!xs?.length || !mainChart) return;
    e.preventDefault();
    const rect = minimapEl.getBoundingClientRect();
    const W     = rect.width;
    const xMin  = xs[0], xMax = xs[xs.length - 1], xRange = xMax - xMin;
    const span  = mainChart.scales.x.max - mainChart.scales.x.min;
    const startClientX = e.clientX;
    const startViewMin = mainChart.scales.x.min;

    function move(ev) {
      const dx    = ev.clientX - startClientX;
      const dUnit = (dx / W) * xRange;
      let newMin  = startViewMin + dUnit;
      let newMax  = newMin + span;
      if (newMin < xMin) { newMin = xMin; newMax = xMin + span; }
      if (newMax > xMax) { newMax = xMax; newMin = xMax - span; }
      mainChart.setScale('x', { min: newMin, max: newMax });
      publishRange();
    }
    function up() {
      window.removeEventListener('pointermove', move);
      window.removeEventListener('pointerup',   up);
    }
    window.addEventListener('pointermove', move);
    window.addEventListener('pointerup',   up);
    move(e); // center on click immediately
  }

  // ─── uPlot main chart ────────────────────────────────────────────────────────
  let currentData = data ?? [[], ...channels.map(() => [])];

  function buildSeries() {
    return channels.map(key => ({
      label: CHANNELS[key]?.name ?? key,
      stroke: CHANNELS[key]?.color ?? '#fff',
      width: 1.5,
      show: visible[key],
    }));
  }

  function buildOpts(w, h) {
    return {
      width: w,
      height: h,
      cursor: {
        show: true,
        drag: { x: true, y: false, uni: 50 },
        // Hovering one panel shows the cursor at the same time in all of them.
        sync: { key: isPlayback ? 'playback' : 'live', setSeries: false },
      },
      legend: { show: false },
      select: { show: true },
      scales: { x: { time: false } },
      axes: [
        {
          stroke: '#555',
          grid:   { stroke: '#1e1e38' },
          ticks:  { stroke: '#2a2a4a' },
          values: (u, vals) => vals.map(v => {
            if (v == null) return '';
            const d = new Date(v * 1000);
            return `${d.getMinutes().toString().padStart(2,'0')}:${d.getSeconds().toString().padStart(2,'0')}`;
          }),
        },
        { stroke: '#555', grid: { stroke: '#1e1e38' }, ticks: { stroke: '#2a2a4a' } },
      ],
      series: [{}, ...buildSeries()],
      hooks: {
        setSelect: [(u) => {
          if (u.select.width > 0) {
            userZoomed = true;
            userScaleChange = true;
            requestAnimationFrame(() => { userScaleChange = false; });
          }
        }],
        setScale: [(u, key) => {
          if (key !== 'x') return;
          if (userScaleChange) {
            userScaleChange = false;
            publishRange();
          }
          computeStats(u.data);
          if (userZoomed) requestAnimationFrame(drawMinimap);
        }],
        // Hover/click in playback mode → seek all panels to this timestamp
        setCursor: [(u) => {
          if (!isPlayback || !dsCtx?.seekToTs) return;
          const idx = u.cursor.idx;
          if (idx == null) return;
          const ts = u.data?.[0]?.[idx];
          if (ts != null) dsCtx.seekToTs(ts);
        }],
        draw: [drawTrackers, drawMarkers, drawSharedCursor],
      },
    };
  }

  onMount(() => {
    mainChart = new uPlot(buildOpts(width, chartH), currentData, mainEl);
  });

  onDestroy(() => mainChart?.destroy());

  // ─── live subscription ───────────────────────────────────────────────────────
  let unsub;
  if (live) {
    unsub = liveBuffer.subscribe(buf => {
      if (!mainChart || buf.count === 0) return;
      const n        = buf.count;
      const oldest   = n < MAX_POINTS ? 0 : buf.head;
      const ts       = new Array(n);
      const chData   = channels.map(() => new Array(n));
      for (let j = 0; j < n; j++) {
        const idx = (oldest + j) % MAX_POINTS;
        ts[j] = buf.ts[idx];
        for (let i = 0; i < channels.length; i++) {
          chData[i][j] = buf.channels[channels[i]][idx];
        }
      }
      currentData = [ts, ...chData];
      mainChart.setData(currentData);
      // Auto-scroll only when not manually zoomed
      if (!userZoomed && ts.length > 0) {
        const last = ts[ts.length - 1];
        mainChart.setScale('x', { min: last - 30, max: last });
      }
      computeStats(currentData);
      if (userZoomed) requestAnimationFrame(drawMinimap);
      if (hasTrackers())  mainChart.redraw();
    });
  }
  onDestroy(() => unsub?.());

  // ─── static data (runs) ──────────────────────────────────────────────────────
  $: if (!live && data && mainChart) {
    currentData = data;
    mainChart.setData(data);
    // setData resets the x scale; keep following the shared zoom if there is one.
    const r = sharedRange && get(sharedRange);
    if (r) {
      applyingShared = true;
      userZoomed = true;
      mainChart.setScale('x', r);
      applyingShared = false;
    }
    computeStats(data);
    requestAnimationFrame(drawMinimap);
  }

  // ─── resize ─────────────────────────────────────────────────────────────────
  $: if (mainChart && chartH) {
    mainChart.setSize({ width, height: chartH });
    requestAnimationFrame(drawMinimap);
  }

  // ─── toggle channel visibility ───────────────────────────────────────────────
  function toggleVisible(key, idx) {
    visible[key] = !visible[key];
    visible = { ...visible };
    mainChart?.setSeries(idx + 1, { show: visible[key] });
    requestAnimationFrame(drawMinimap);
  }

  function toggleTracker(key, stat) {
    trackerOn[key][stat] = !trackerOn[key][stat];
    trackerOn = { ...trackerOn };
    mainChart?.redraw();
  }

  function fmt(v) { return v == null ? '—' : v.toFixed(2); }
</script>

<div class="ts-panel">

  <!-- ── chart area ─────────────────────────────────────────────────────── -->
  <div class="chart-area">
    {#if userZoomed}
      <button class="reset-btn" on:click={resetZoom} title="Reset zoom">↺ Reset</button>
    {/if}
    <div bind:this={mainEl}></div>
  </div>

  <!-- ── minimap ────────────────────────────────────────────────────────── -->
  {#if userZoomed}
    <div class="minimap-wrap">
      <canvas
        bind:this={minimapEl}
        {width}
        height={MINIMAP_H}
        on:pointerdown={onMinimapPointer}
        style="cursor: ew-resize"
      ></canvas>
    </div>
  {/if}

  <!-- ── legend ─────────────────────────────────────────────────────────── -->
  <div class="legend">
    <button class="leg-head" on:click={() => legendOpen = !legendOpen}>
      <span class="leg-caret">{legendOpen ? '▾' : '▸'}</span>
      <span>Channels</span>
    </button>

    {#if legendOpen}
      {#each channels as key, i}
        {@const meta = CHANNELS[key] ?? {}}
        {@const s    = stats[key]}
        {@const t    = trackerOn[key]}
        <div class="leg-row" class:muted={!visible[key]}>

          <!-- dot = visibility toggle -->
          <button
            class="vis-btn"
            style="color: {visible[key] ? (meta.color ?? '#aaa') : '#333'}"
            on:click={() => toggleVisible(key, i)}
            title="{visible[key] ? 'Hide' : 'Show'} channel"
          >●</button>

          <span class="leg-name">{meta.name ?? key}</span>
          <span class="leg-unit">{meta.unit ?? ''}</span>

          <!-- stat toggles -->
          <div class="stat-pills">
            {#each ['min','max','avg'] as st}
              <button
                class="stat-pill"
                class:on={t[st]}
                style="--c: {meta.color ?? '#4FC3F7'}"
                on:click={() => toggleTracker(key, st)}
                title="Toggle {st} tracker line"
              >
                {st}
                {#if s}
                  <span class="pill-val">{fmt(s[st])}</span>
                {/if}
              </button>
            {/each}
          </div>

        </div>
      {/each}
    {/if}
  </div>

</div>

<style>
  .ts-panel {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
  }

  /* ── chart ───────────────────────────────────────────────────────────────── */
  .chart-area {
    position: relative;
    flex-shrink: 0;
  }
  .reset-btn {
    position: absolute;
    top: 4px;
    right: 6px;
    z-index: 10;
    background: #1e1e38;
    border: 1px solid #3a3a5a;
    color: #4FC3F7;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 0.72rem;
    cursor: pointer;
    font-weight: 600;
  }
  .reset-btn:hover { background: #2a2a4a; }

  /* uPlot selection box styling */
  :global(.u-select) {
    background: rgba(79, 195, 247, 0.10) !important;
    border: 1px solid rgba(79, 195, 247, 0.5) !important;
  }

  /* ── minimap ─────────────────────────────────────────────────────────────── */
  .minimap-wrap {
    flex-shrink: 0;
    border-top: 1px solid #1e1e38;
    overflow: hidden;
    background: #0d0d20;
  }
  .minimap-wrap canvas {
    display: block;
    width: 100%;
  }

  /* ── legend ──────────────────────────────────────────────────────────────── */
  .legend {
    flex-shrink: 0;
    border-top: 1px solid #1e1e38;
    background: #0f0f1e;
  }
  .leg-head {
    display: flex;
    align-items: center;
    gap: 5px;
    width: 100%;
    background: none;
    border: none;
    color: #666;
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    padding: 4px 8px;
    cursor: pointer;
    text-align: left;
  }
  .leg-head:hover { color: #aaa; }
  .leg-caret { font-size: 0.65rem; }

  .leg-row {
    display: flex;
    align-items: center;
    gap: 5px;
    padding: 1px 8px;
    height: 26px;
    transition: opacity 0.15s;
  }
  .leg-row.muted { opacity: 0.4; }

  .vis-btn {
    background: none;
    border: none;
    padding: 0;
    cursor: pointer;
    font-size: 0.7rem;
    line-height: 1;
    flex-shrink: 0;
  }

  .leg-name {
    font-size: 0.72rem;
    color: #bbb;
    flex: 1;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .leg-unit {
    font-size: 0.65rem;
    color: #555;
    width: 18px;
    text-align: right;
    flex-shrink: 0;
  }

  /* ── stat pills ──────────────────────────────────────────────────────────── */
  .stat-pills {
    display: flex;
    gap: 3px;
    flex-shrink: 0;
  }
  .stat-pill {
    display: flex;
    align-items: center;
    gap: 3px;
    padding: 1px 5px;
    border-radius: 3px;
    border: 1px solid #2a2a4a;
    background: none;
    color: #555;
    font-size: 0.65rem;
    cursor: pointer;
    white-space: nowrap;
    transition: all 0.1s;
    line-height: 1.4;
  }
  .stat-pill:hover { border-color: var(--c); color: #aaa; }
  .stat-pill.on {
    border-color: var(--c);
    color: var(--c);
    background: color-mix(in srgb, var(--c) 12%, transparent);
  }
  .pill-val {
    font-family: monospace;
    font-size: 0.68rem;
    opacity: 0.85;
  }
</style>
