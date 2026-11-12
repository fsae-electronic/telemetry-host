// Detects notable changes between consecutive samples: main ECU restarts and
// faults, lost frames, driver state changes, stale CAN messages, tractive
// system and TPS problems. Runs on live packets (Health tab) and on saved runs
// (run view → Events). Samples already include the derived channels and the
// latest slow-frame values.
import {
  ACTIVE_FAULT_BITS, DRIVER_LABELS, TS_STATE_BITS, VALID_BITS, canErrorLevel,
  cia402State, driverSeen, ecuMode, errorClass, firmwareLabel, hex, isNmtState, nmtState, resetCause,
} from './diagnostics.js';

// A frame interval above this is reported as a gap (frames are sent every 100 ms).
export const LINK_GAP_MS = 500;
const CPU_LOAD_WARN = 80; // %

/**
 * @typedef {{ts: number, level: 'info'|'warn'|'error', kind: string, text: string, value?: number}} TelemetryEvent
 */

// createEventDetector returns detect(ts, values) → TelemetryEvent[]. Make a
// new one per session.
export function createEventDetector() {
  let prev = null;

  return function detect(ts, v) {
    const events = [];
    const emit = (level, kind, text, value) => events.push({ ts, level, kind, text, value });
    const rose = (key, bit) => has(v[key], bit) && !has(prev[key], bit);
    const fell = (key, bit) => !has(v[key], bit) && has(prev[key], bit);

    if (v.frame_dt != null && v.frame_dt > LINK_GAP_MS) {
      emit('warn', 'gap', `No data for ${(v.frame_dt / 1000).toFixed(1)} s`, v.frame_dt);
    }

    if (prev) {
      const restarted = v.ecu_time != null && prev.ecu_time != null && v.ecu_time < prev.ecu_time;
      if (restarted) {
        emit('warn', 'ecu', `Main ECU restarted (was up ${prev.ecu_time.toFixed(1)} s)`);
      } else if (v.seq != null && prev.seq != null) {
        const lost = (v.seq - prev.seq - 1 + 256) % 256;
        if (lost > 0) emit('warn', 'lost', `${lost} frame${lost > 1 ? 's' : ''} lost`, lost);
      }

      // Main ECU identity and health (slow frame, 1 Hz).
      const fw = firmwareLabel(v);
      const fwBefore = firmwareLabel(prev);
      if (fw && (!fwBefore || (v.uptime_s ?? 0) < (prev.uptime_s ?? 0))) {
        emit('info', 'ecu', `Main ECU ${fw}, reset #${v.reset_count ?? 0}: ${resetCause(v.last_reset_cause)}`);
      } else if (fw && fwBefore && fw !== fwBefore) {
        emit('info', 'ecu', `Main ECU firmware changed: ${fwBefore} → ${fw}`);
      }
      for (const [bit, name] of ACTIVE_FAULT_BITS) {
        if (rose('active_faults', bit)) emit('error', 'ecu', `Main ECU fault: ${name}`);
        else if (fell('active_faults', bit)) emit('info', 'ecu', `Main ECU fault cleared: ${name}`);
      }
      if ((v.loop_overruns ?? 0) > (prev.loop_overruns ?? 0) && !restarted) {
        const n = v.loop_overruns - (prev.loop_overruns ?? 0);
        emit('warn', 'ecu', `Main ECU loop overrun ×${n} (worst loop ${v.loop_time_max} µs)`, n);
      }
      if ((v.cpu_load ?? 0) >= CPU_LOAD_WARN && (prev.cpu_load ?? 0) < CPU_LOAD_WARN) {
        emit('warn', 'ecu', `Main ECU CPU load ${v.cpu_load} %`, v.cpu_load);
      }
      for (const [key, label] of [['can_tx_err', 'TX'], ['can_rx_err', 'RX']]) {
        const level = canErrorLevel(v[key] ?? 0);
        if (level !== canErrorLevel(prev[key] ?? 0)) {
          const text = { ok: 'back to normal', warn: 'warning level', error: 'error passive' }[level];
          emit(level === 'ok' ? 'info' : level, 'can', `CAN ${label} errors ${v[key]}: ${text}`, v[key]);
        }
      }
      if ((v.can_busoff_count ?? 0) > (prev.can_busoff_count ?? 0) && !restarted) {
        emit('error', 'can', 'CAN bus-off');
      }

      // Main ECU state.
      if (v.ecu_state != null && prev.ecu_state != null) {
        if (ecuMode(v.ecu_state) !== ecuMode(prev.ecu_state)) {
          emit('info', 'ecu', `Mode ${ecuMode(prev.ecu_state)} → ${ecuMode(v.ecu_state)}`);
        }
        if (rose('ecu_state', 2)) emit('info', 'ecu', 'Drive enabled');
        if (fell('ecu_state', 2)) emit('info', 'ecu', 'Drive disabled');
        if (rose('ecu_state', 3)) emit('info', 'ecu', 'Ready to drive');
        if (rose('ecu_state', 4)) emit('info', 'ecu', 'Traction control on');
        if (fell('ecu_state', 4)) emit('info', 'ecu', 'Traction control off');
        if (rose('ecu_state', 7)) emit('error', 'ecu', 'ECU: APPS implausible');
        if (fell('ecu_state', 7)) emit('info', 'ecu', 'ECU: APPS plausible again');
        if (rose('ecu_state', 8)) emit('warn', 'ecu', 'ECU: brake/throttle plausibility (EV.4.7) active');
        if (fell('ecu_state', 8)) emit('info', 'ecu', 'ECU: EV.4.7 cleared');
        if (rose('ecu_state', 10)) emit('error', 'ecu', 'ECU safe mode: torque forced to 0');
        if (fell('ecu_state', 10)) emit('info', 'ecu', 'ECU left safe mode');
      }

      // CAN messages: first received, lost after an ECU reset, stopped arriving.
      const mask = v.valid_mask ?? 0;
      const prevMask = prev.valid_mask ?? 0;
      const lostBits = prevMask & ~mask;
      const gained = mask & ~prevMask;
      if (lostBits && !restarted) {
        emit('warn', 'ecu', `valid_mask lost ${bitList(lostBits)} — main ECU reset?`);
      }
      if (gained) {
        emit('info', 'ecu', `First received: ${bitList(gained)}`);
      }
      if (v.fresh_mask != null && prev.fresh_mask != null) {
        const stillValid = mask & prevMask;
        const stale = stillValid & prev.fresh_mask & ~v.fresh_mask;
        const back = stillValid & v.fresh_mask & ~prev.fresh_mask;
        if (stale) emit('warn', 'stale', `Stopped arriving: ${bitList(stale)}`);
        if (back) emit('info', 'stale', `Arriving again: ${bitList(back)}`);
      }

      if (v.calibration_type !== prev.calibration_type) {
        emit('info', 'ecu', `Calibration command ${prev.calibration_type} → ${v.calibration_type}`);
      }

      // Tractive system and accumulator (only when the ECU reports them).
      if (has(mask, 20) && has(prevMask, 20)) {
        if (fell('ts_state', 0)) {
          emit('error', 'ts', `Shutdown circuit opened${v.sdc_open_segment ? ` at segment ${v.sdc_open_segment}` : ''}`);
        }
        if (rose('ts_state', 0)) emit('info', 'ts', 'Shutdown circuit closed');
        for (const bit of [5, 6, 7]) {
          if (fell('ts_state', bit)) emit('error', 'ts', `${TS_STATE_BITS.find(b => b[0] === bit)[1].replace(' ok', '')} fault`);
        }
        if (rose('ts_state', 8)) emit('info', 'ts', 'Tractive system active');
        if (fell('ts_state', 8)) emit('warn', 'ts', 'Tractive system off');
      }
      if (has(mask, 15) && (v.bms_fault ?? 0) !== (prev.bms_fault ?? 0)) {
        if (v.bms_fault) emit('warn', 'bms', `BMS faults ${hex(v.bms_fault)}`, v.bms_fault);
        else emit('info', 'bms', 'BMS faults cleared');
      }

      if (v.tps_implausible === 1 && prev.tps_implausible !== 1) {
        emit('error', 'tps', `TPS implausible (TPS1 ${fmt(v.tps1)} %, TPS2 ${fmt(v.tps2)} %)`, v.tps_diff);
      } else if (v.tps_implausible === 0 && prev.tps_implausible === 1) {
        emit('info', 'tps', 'TPS plausible again');
      }

      for (let n = 0; n < 2; n++) {
        if (!driverSeen(mask, n)) continue;
        const label = DRIVER_LABELS[n];
        const p = `drv${n}_`;

        const state = cia402State(v[p + 'status_word']);
        const prevState = cia402State(prev[p + 'status_word']);
        if (state.name !== prevState.name) {
          const level = state.level === 'error' ? 'error' : state.level === 'warn' ? 'warn' : 'info';
          emit(level, 'driver', `${label}: ${prevState.name} → ${state.name}`);
        }

        const err = v[p + 'error'] ?? 0;
        if (err !== (prev[p + 'error'] ?? 0)) {
          if (err) emit('error', 'driver', `${label} error ${hex(err)} (${errorClass(err)})`, err);
          else emit('info', 'driver', `${label} error cleared`);
        }

        const warn = v[p + 'warnings'] ?? 0;
        if (warn !== (prev[p + 'warnings'] ?? 0)) {
          if (warn) emit('warn', 'driver', `${label} warnings ${hex(warn)}`, warn);
          else emit('info', 'driver', `${label} warnings cleared`);
        }

        // Only transitions into a known NMT state, so a byte that is not an
        // NMT state (e.g. a counter) cannot flood the log.
        const hb = v[p + 'heartbeat_state'];
        if (hb !== prev[p + 'heartbeat_state'] && isNmtState(hb)) {
          emit(hb === 5 ? 'info' : 'warn', 'driver', `${label} NMT: ${nmtState(prev[p + 'heartbeat_state'])} → ${nmtState(hb)}`);
        }
      }
    }

    prev = v;
    return events;
  };
}

function has(v, bit) {
  return ((v ?? 0) & (1 << bit)) !== 0;
}

function fmt(v) {
  return v == null ? '—' : Number.isInteger(v) ? v : v.toFixed(1);
}

function bitList(bits) {
  return VALID_BITS.filter(b => bits & (1 << b.bit)).map(b => `${b.group} ${b.name}`).join(', ');
}
