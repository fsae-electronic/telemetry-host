<script>
  import { onDestroy } from 'svelte';
  import { status, link, liveValues, latestPacket, liveEvents, clearLiveEvents } from '../lib/ws.js';
  import {
    VALID_BITS, DRIVER_LABELS, ECU_STATE_BITS, TORQUE_LIMIT_BITS, TS_STATE_BITS, ACTIVE_FAULT_BITS,
    cia402State, statusFlags, nmtState, errorClass, setBits, hex, driverSeen, ecuMode, bitNames,
    resetCause, firmwareLabel, canErrorLevel,
  } from '../lib/diagnostics.js';
  import { TPS_MAX_DIFF } from '../lib/derived.js';
  import { fmtValue } from '../lib/runs.js';
  import TimeSeriesPanel from '../lib/components/panels/TimeSeriesPanel.svelte';

  const EXPECTED_HZ = 10;

  // Ticks so "last frame" ages even when no packets arrive.
  let now = Date.now() / 1000;
  const timer = setInterval(() => { now = Date.now() / 1000; }, 250);
  onDestroy(() => clearInterval(timer));

  $: v = $liveValues;
  $: hasData = !!$latestPacket;
  $: age = hasData ? Math.max(0, now - $latestPacket.ts) : null;
  $: mask = v.valid_mask ?? 0;
  $: fresh = v.fresh_mask ?? 0;
  const bit = (value, b) => ((value ?? 0) & (1 << b)) !== 0;

  // ── link ────────────────────────────────────────────────────────────────────
  $: gaps = $liveEvents.filter(e => e.kind === 'gap');
  $: maxGap = gaps.reduce((m, e) => Math.max(m, e.value ?? 0), 0);
  $: linkLevel = !$status.connected ? 'idle'
    : age == null || age > 1 ? 'error'
    : ($link?.rate_hz ?? EXPECTED_HZ) < EXPECTED_HZ * 0.8 || $link?.lost > 0 || $link?.crc_errors > 0 ? 'warn'
    : 'ok';
  $: linkText = { idle: 'Disconnected', error: 'No data', warn: 'Degraded', ok: 'OK' }[linkLevel];

  // ── main ECU ────────────────────────────────────────────────────────────────
  $: firmware = firmwareLabel(v);
  $: ecuState = v.ecu_state ?? 0;
  $: stateChips = bitNames(ecuState, ECU_STATE_BITS);
  $: limitChips = bitNames(v.torque_limit_reason, TORQUE_LIMIT_BITS);
  $: faults = bitNames(v.active_faults, ACTIVE_FAULT_BITS);
  $: ecuLevel = !hasData ? 'idle'
    : bit(ecuState, 10) || faults.length ? 'error'
    : canErrorLevel(v.can_tx_err ?? 0) !== 'ok' || canErrorLevel(v.can_rx_err ?? 0) !== 'ok' || (v.cpu_load ?? 0) >= 80 ? 'warn'
    : 'ok';
  $: ecuText = { idle: 'No data', error: bit(ecuState, 10) ? 'Safe mode' : 'Fault', warn: 'Check', ok: 'OK' }[ecuLevel];

  // ── TPS ─────────────────────────────────────────────────────────────────────
  $: tpsLevel = !hasData || !bit(mask, 0) ? 'idle' : v.tps_implausible || bit(ecuState, 7) ? 'error' : 'ok';
  $: tpsEvents = $liveEvents.filter(e => e.kind === 'tps' && e.level === 'error').length;

  // ── CAN messages: fresh (arriving), stale (stopped), never received ─────────
  $: maskGroups = [...new Set(VALID_BITS.map(b => b.group))].map(group => ({
    group,
    bits: VALID_BITS.filter(b => b.group === group),
  }));
  // Takes the masks as arguments so the template re-renders when they change.
  function lightState(b, valid, fresh) {
    if (!bit(valid, b)) return 'never';
    return bit(fresh, b) ? 'fresh' : 'stale';
  }

  // ── accumulator / tractive system ───────────────────────────────────────────
  $: bmsValid = bit(mask, 15);
  $: tsValid = bit(mask, 20);
  $: tsChips = TS_STATE_BITS.map(([b, name]) => ({ name, on: bit(v.ts_state, b) }));

  // ── drivers ─────────────────────────────────────────────────────────────────
  $: drivers = [0, 1].map(n => {
    const p = `drv${n}_`;
    return {
      label: DRIVER_LABELS[n],
      seen: driverSeen(mask, n),
      state: cia402State(v[p + 'status_word']),
      flags: statusFlags(v[p + 'status_word']),
      statusWord: v[p + 'status_word'],
      controlWord: v[p + 'control_word'],
      nmt: v[p + 'heartbeat_state'],
      error: v[p + 'error'] ?? 0,
      warnings: v[p + 'warnings'] ?? 0,
      rows: [
        ['Torque cmd (ECU)', v[n ? 'torque_cmd_r' : 'torque_cmd_l'], '‰'],
        ['Target torque', v[p + 'target_torque'], '‰'],
        ['Torque actual', v[p + 'torque_actual'], '‰'],
        ['Current actual', v[p + 'current_actual'], '‰'],
        ['Motor velocity', v[p + 'motor_velocity'], 'rpm'],
        ['DC voltage', v[p + 'driver_dc_voltage'], 'V'],
        ['Driver temp', v[p + 'driver_temp'], '°C'],
        ['Motor temp', v[p + 'motor_temp'], '°C'],
      ],
    };
  });

  // ── event log ───────────────────────────────────────────────────────────────
  let levels = { error: true, warn: true, info: true };
  $: shownEvents = [...$liveEvents].reverse().filter(e => levels[e.level]);

  function fmtClock(ts) {
    const d = new Date(ts * 1000);
    return d.toLocaleTimeString([], { hour12: false }) + '.' + Math.floor(d.getMilliseconds() / 100);
  }

  function fmtUptime(s) {
    if (s == null) return '—';
    const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60);
    return h ? `${h} h ${m} min` : m ? `${m} min ${Math.floor(s % 60)} s` : `${Math.floor(s)} s`;
  }

  let chartW = 400;
</script>

<div class="health">
  {#if !$status.connected}
    <div class="banner">Not connected. Connect to the serial port, or pick SIMULATOR, to see live diagnostics.</div>
  {/if}

  <div class="grid">
    <!-- link ─────────────────────────────────────────────────────────────── -->
    <section class="card">
      <header>
        <h3>Link</h3>
        <span class="badge {linkLevel}">{linkText}</span>
      </header>
      <dl>
        <dt>Last frame</dt>
        <dd class:bad={age != null && age > 1}>{age == null ? '—' : age < 1 ? `${Math.round(age * 1000)} ms ago` : `${age.toFixed(1)} s ago`}</dd>
        <dt>Frame rate</dt>
        <dd class:bad={$link && $status.connected && $link.rate_hz < EXPECTED_HZ * 0.8}>{$link ? $link.rate_hz.toFixed(0) : '—'} / {EXPECTED_HZ} Hz</dd>
        <dt>Frames (fast / slow)</dt>
        <dd>{$link ? `${$link.frames} / ${$link.slow_frames}` : '—'}</dd>
        <dt>Frames lost (seq)</dt>
        <dd class:bad={$link?.lost > 0}>{$link?.lost ?? '—'}</dd>
        <dt>CRC errors</dt>
        <dd class:bad={$link?.crc_errors > 0}>{$link?.crc_errors ?? '—'}</dd>
        <dt>Gaps &gt; 0.5 s</dt>
        <dd class:bad={gaps.length > 0}>{gaps.length}{gaps.length ? ` (max ${(maxGap / 1000).toFixed(1)} s)` : ''}</dd>
        <dt>Discarded bytes</dt>
        <dd class:bad={$link?.discarded_bytes > 0}>{$link?.discarded_bytes ?? '—'}</dd>
        <dt>Dropped (host slow)</dt>
        <dd class:bad={$link?.dropped > 0}>{$link?.dropped ?? '—'}</dd>
      </dl>
      <p class="note">Counters since the last connect. The main ECU's boot message shows up as discarded bytes.</p>
    </section>

    <!-- main ECU ─────────────────────────────────────────────────────────── -->
    <section class="card">
      <header>
        <h3>Main ECU</h3>
        <span class="badge {ecuLevel}">{ecuText}</span>
      </header>
      <dl>
        <dt>Firmware</dt>
        <dd class="mono">{firmware ?? '—'}</dd>
        <dt>Uptime</dt>
        <dd>{fmtUptime(v.uptime_s)}</dd>
        <dt>Last reset</dt>
        <dd>{v.reset_count == null ? '—' : `#${v.reset_count} · ${resetCause(v.last_reset_cause)}`}</dd>
        <dt>Mode</dt>
        <dd>{hasData ? ecuMode(ecuState) : '—'}</dd>
        <dt>CPU load</dt>
        <dd class:warn={(v.cpu_load ?? 0) >= 80}>{fmtValue(v.cpu_load)} %</dd>
        <dt>Loop overruns</dt>
        <dd class:warn={v.loop_overruns > 0}>{fmtValue(v.loop_overruns)} <span class="dim">worst {fmtValue(v.loop_time_max)} µs</span></dd>
        <dt>CAN TX / RX errors</dt>
        <dd>
          <span class="lvl-{canErrorLevel(v.can_tx_err ?? 0)}">{fmtValue(v.can_tx_err)}</span> /
          <span class="lvl-{canErrorLevel(v.can_rx_err ?? 0)}">{fmtValue(v.can_rx_err)}</span>
          <span class="dim">bus-off {fmtValue(v.can_busoff_count)}</span>
        </dd>
      </dl>
      <div class="chip-block">
        <span class="chip-label">State</span>
        <div class="chips">
          {#each stateChips as c}<span class="chip" class:bad={['APPS implausible', 'Brake/throttle (EV.4.7)', 'Safe mode'].includes(c)}>{c}</span>{:else}<span class="dim">—</span>{/each}
        </div>
      </div>
      <div class="chip-block">
        <span class="chip-label">Torque limited by</span>
        <div class="chips">
          {#each limitChips as c}<span class="chip warn">{c}</span>{:else}<span class="dim">Nothing</span>{/each}
        </div>
      </div>
      <div class="chip-block">
        <span class="chip-label">Active faults</span>
        <div class="chips">
          {#each faults as c}<span class="chip bad">{c}</span>{:else}<span class="dim">None</span>{/each}
        </div>
      </div>
    </section>

    <!-- TPS ──────────────────────────────────────────────────────────────── -->
    <section class="card">
      <header>
        <h3>Pedal plausibility</h3>
        <span class="badge {tpsLevel}">{tpsLevel === 'idle' ? 'No TPS' : tpsLevel === 'error' ? 'Implausible' : 'OK'}</span>
      </header>
      <div class="tps">
        {#each [['TPS 1', v.tps1], ['TPS 2', v.tps2]] as [name, val]}
          <div class="tps-row">
            <span class="tps-name">{name}</span>
            <div class="bar"><div class="fill" style="width: {Math.max(0, Math.min(100, val ?? 0))}%"></div></div>
            <span class="tps-val">{fmtValue(val)} %</span>
          </div>
        {/each}
      </div>
      <dl>
        <dt>Difference</dt>
        <dd class:bad={v.tps_diff > TPS_MAX_DIFF}>{fmtValue(v.tps_diff)} % (limit {TPS_MAX_DIFF} %)</dd>
        <dt>ECU says</dt>
        <dd class:bad={bit(ecuState, 7)}>{bit(ecuState, 7) ? 'APPS implausible' : 'Plausible'}</dd>
        <dt>Torque request</dt>
        <dd>{fmtValue(v.torque_request)} ‰</dd>
        <dt>Implausibilities</dt>
        <dd class:bad={tpsEvents > 0}>{tpsEvents} this session</dd>
      </dl>
      <p class="note">FSAE T.4.2: more than {TPS_MAX_DIFF} points between sensors for over 100 ms. Checked by the host and by the ECU.</p>
    </section>

    <!-- CAN messages ─────────────────────────────────────────────────────── -->
    <section class="card">
      <header>
        <h3>CAN messages</h3>
        <span class="mono dim">valid {hex(mask, 6)} · fresh {hex(fresh, 6)}</span>
      </header>
      {#each maskGroups as g}
        <div class="mask-group">
          <span class="mask-label">{g.group}</span>
          <div class="lights">
            {#each g.bits as b}
              <span class="light {lightState(b.bit, mask, fresh)}" title="bit {b.bit}{b.title ? ` — ${b.title}` : ''}">{b.name}</span>
            {/each}
          </div>
        </div>
      {/each}
      <p class="note"><span class="key fresh">arriving</span> <span class="key stale">stopped arriving</span> <span class="key never">never received since the ECU booted</span></p>
    </section>

    <!-- accumulator / tractive system ────────────────────────────────────── -->
    <section class="card">
      <header>
        <h3>Accumulator &amp; tractive system</h3>
        {#if tsValid}
          <span class="badge {bit(v.ts_state, 0) ? 'ok' : 'error'}">{bit(v.ts_state, 0) ? 'SDC closed' : 'SDC open'}</span>
        {/if}
      </header>
      {#if bmsValid}
        <dl>
          <dt>Pack</dt>
          <dd>{fmtValue(v.pack_voltage)} V · {fmtValue(v.pack_current)} A</dd>
          <dt>State of charge</dt>
          <dd>{fmtValue(v.soc)} %</dd>
          <dt>Cells</dt>
          <dd>{fmtValue(v.cell_v_min)}–{fmtValue(v.cell_v_max)} V · max {fmtValue(v.cell_temp_max)} °C</dd>
          <dt>BMS current limit</dt>
          <dd>{fmtValue(v.bms_current_limit)} A</dd>
          <dt>BMS faults</dt>
          <dd class:bad={v.bms_fault}>{v.bms_fault ? hex(v.bms_fault) : 'None'}</dd>
        </dl>
      {:else}
        <p class="dim">No BMS data: the main ECU does not send it yet (valid bit 15 off).</p>
      {/if}
      {#if tsValid}
        <div class="chips ts-chips">
          {#each tsChips as c}<span class="chip" class:on={c.on}>{c.name}</span>{/each}
        </div>
        {#if v.sdc_open_segment}<p class="note bad-text">Shutdown circuit open at segment {v.sdc_open_segment}</p>{/if}
      {:else}
        <p class="dim">No tractive system data (valid bit 20 off).</p>
      {/if}
      {#if bit(mask, 19)}
        <p class="note">LV supply {fmtValue(v.lv_voltage)} V{bit(mask, 18) ? ` · coolant ${fmtValue(v.coolant_temp)} °C` : ''}</p>
      {/if}
    </section>

    <!-- drivers ──────────────────────────────────────────────────────────── -->
    {#each drivers as d}
      <section class="card">
        <header>
          <h3>{d.label}</h3>
          <span class="badge {d.seen ? d.state.level : 'idle'}">{d.seen ? d.state.name : 'Not seen'}</span>
        </header>
        <dl>
          <dt>NMT</dt>
          <dd class:bad={d.seen && d.nmt !== 5}>{d.seen ? nmtState(d.nmt) : '—'}</dd>
          <dt>Status word</dt>
          <dd class="mono">{hex(d.statusWord)} <span class="dim">{d.flags.join(', ')}</span></dd>
          <dt>Control word</dt>
          <dd class="mono">{hex(d.controlWord)}</dd>
          <dt>Error</dt>
          <dd class:bad={d.error}>{#if d.error}<span class="mono">{hex(d.error)}</span> {errorClass(d.error)}{:else}None{/if}</dd>
          <dt>Warnings</dt>
          <dd class:warn={d.warnings}>{#if d.warnings}<span class="mono">{hex(d.warnings)}</span> bits {setBits(d.warnings).join(', ')}{:else}None{/if}</dd>
          {#each d.rows as [name, val, unit]}
            <dt>{name}</dt>
            <dd>{fmtValue(val)} <span class="dim">{unit}</span></dd>
          {/each}
        </dl>
      </section>
    {/each}

    <!-- frame interval ───────────────────────────────────────────────────── -->
    <section class="card wide">
      <header><h3>Frame interval</h3><span class="dim">ms between frames, expected 100</span></header>
      <div class="chart" bind:clientWidth={chartW}>
        <TimeSeriesPanel channels={['frame_dt']} width={chartW} height={200} live={true} />
      </div>
    </section>

    <!-- event log ────────────────────────────────────────────────────────── -->
    <section class="card wide">
      <header>
        <h3>Events</h3>
        <div class="log-tools">
          {#each ['error', 'warn', 'info'] as lv}
            <label class="lv-{lv}"><input type="checkbox" bind:checked={levels[lv]} /> {lv}</label>
          {/each}
          <button class="btn-ghost" on:click={clearLiveEvents}>Clear</button>
        </div>
      </header>
      <div class="log">
        {#if !shownEvents.length}
          <p class="dim">No events yet.</p>
        {/if}
        {#each shownEvents as e}
          <div class="log-row lv-{e.level}">
            <span class="mono dim">{fmtClock(e.ts)}</span>
            <span>{e.text}</span>
          </div>
        {/each}
      </div>
    </section>
  </div>
</div>

<style>
  .health { height: 100%; overflow-y: auto; padding: 12px; }
  .banner {
    padding: 8px 12px;
    margin-bottom: 12px;
    border: 1px solid #2a2a4a;
    border-radius: 6px;
    color: #888;
    font-size: 0.82rem;
    background: #12122a;
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
    gap: 12px;
    align-items: start;
  }
  .card {
    background: #12122a;
    border: 1px solid #2a2a4a;
    border-radius: 6px;
    padding: 10px 12px;
    min-width: 0;
  }
  .card.wide { grid-column: 1 / -1; }
  header { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 8px; }
  h3 { margin: 0; font-size: 0.78rem; color: #aaa; text-transform: uppercase; letter-spacing: 0.05em; }

  .badge {
    font-size: 0.7rem;
    font-weight: 700;
    padding: 2px 8px;
    border-radius: 10px;
    border: 1px solid;
    white-space: nowrap;
  }
  .badge.ok    { color: #81C784; border-color: #2e5e30; background: #12301a; }
  .badge.warn  { color: #FFB74D; border-color: #6b4a1a; background: #2e2010; }
  .badge.error { color: #ff8a80; border-color: #7a2a2a; background: #351414; }
  .badge.idle  { color: #888;    border-color: #333;    background: #1a1a2e; }

  dl { display: grid; grid-template-columns: auto 1fr; gap: 3px 12px; margin: 0; font-size: 0.78rem; }
  dt { color: #777; }
  dd { margin: 0; color: #ddd; text-align: right; }
  dd.bad { color: #ff8a80; }
  dd.warn { color: #FFB74D; }
  .lvl-ok { color: #ddd; }
  .lvl-warn { color: #FFB74D; }
  .lvl-error { color: #ff8a80; }
  .mono { font-family: monospace; }
  .dim { color: #666; font-size: 0.72rem; }
  p.dim { margin: 4px 0; }
  .note { font-size: 0.68rem; color: #555; margin: 8px 0 0; }
  .bad-text { color: #ff8a80; }

  .chip-block { margin-top: 8px; display: flex; flex-direction: column; gap: 3px; }
  .chip-label { font-size: 0.66rem; color: #666; text-transform: uppercase; letter-spacing: 0.05em; }
  .chips { display: flex; flex-wrap: wrap; gap: 4px; }
  .ts-chips { margin-top: 8px; }
  .chip {
    font-size: 0.7rem;
    padding: 1px 7px;
    border-radius: 3px;
    border: 1px solid #2e4a5e;
    color: #9fc8e6;
    background: #13202e;
  }
  .chip.warn { border-color: #6b4a1a; color: #FFB74D; background: #2e2010; }
  .chip.bad  { border-color: #7a2a2a; color: #ff8a80; background: #351414; }
  .ts-chips .chip { border-color: #333; color: #555; background: #16162a; }
  .ts-chips .chip.on { border-color: #2e5e30; color: #a5d6a7; background: #12301a; }

  .tps { display: flex; flex-direction: column; gap: 5px; margin-bottom: 8px; }
  .tps-row { display: flex; align-items: center; gap: 8px; font-size: 0.78rem; }
  .tps-name { width: 42px; color: #888; }
  .bar { flex: 1; height: 10px; background: #1e1e38; border-radius: 3px; overflow: hidden; }
  .fill { height: 100%; background: #FFB74D; }
  .tps-val { width: 64px; text-align: right; font-family: monospace; color: #ddd; }

  .mask-group { margin-bottom: 7px; }
  .mask-label { display: block; font-size: 0.66rem; color: #666; text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 3px; }
  .lights { display: flex; flex-wrap: wrap; gap: 4px; }
  .light, .key {
    font-size: 0.7rem;
    padding: 2px 7px;
    border-radius: 3px;
    border: 1px solid #333;
    color: #555;
    background: #16162a;
  }
  .key { font-size: 0.64rem; padding: 0 5px; margin-right: 4px; }
  .light.fresh, .key.fresh { color: #a5d6a7; border-color: #2e5e30; background: #12301a; }
  .light.stale, .key.stale { color: #FFB74D; border-color: #6b4a1a; background: #2e2010; }

  .chart { width: 100%; height: 230px; }

  .log-tools { display: flex; align-items: center; gap: 10px; font-size: 0.72rem; }
  .log-tools label { display: flex; align-items: center; gap: 3px; cursor: pointer; }
  .btn-ghost {
    background: none; border: 1px solid #2a2a4a; color: #888;
    padding: 2px 8px; border-radius: 4px; cursor: pointer; font-size: 0.72rem;
  }
  .btn-ghost:hover { color: #ccc; border-color: #4a4a6a; }
  .log { max-height: 320px; overflow-y: auto; display: flex; flex-direction: column; gap: 2px; }
  .log-row {
    display: flex;
    gap: 10px;
    font-size: 0.76rem;
    padding: 2px 6px;
    border-left: 2px solid #455A64;
    color: #b0bec5;
  }
  .lv-error { color: #e57373; }
  .lv-warn  { color: #FFB74D; }
  .lv-info  { color: #90A4AE; }
  .log-row.lv-error { border-left-color: #e57373; color: #f2b8b8; }
  .log-row.lv-warn  { border-left-color: #FFB74D; color: #f5d9b0; }
</style>
