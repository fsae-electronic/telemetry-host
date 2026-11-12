// Loading saved runs and editing their metadata and markers.
import { apiGet, apiPost, apiPatch, apiDelete } from './ws.js';
import { ALL_CHANNEL_KEYS, SLOW_KEYS } from './channels.js';
import { createDeriver } from './derived.js';
import { createEventDetector } from './events.js';

// Editable run metadata fields, in display order.
export const META_FIELDS = [
  { key: 'driver',  label: 'Driver' },
  { key: 'mode',    label: 'Mode',    options: ['normal', 'race'] },
  { key: 'track',   label: 'Track / test' },
  { key: 'weather', label: 'Weather' },
  { key: 'setup',   label: 'Setup',   placeholder: 'Tire pressures, map, ballast…' },
];

/**
 * Loads a run's samples, adds the derived channels and detects events.
 * samples is uPlot-format data: [ts[], ...one array per ALL_CHANNEL_KEYS].
 * @returns {Promise<{samples: any[][], events: import('./events.js').TelemetryEvent[]}>}
 */
export async function loadRunData(runId) {
  const raw = await apiGet(`/api/runs/${runId}/samples`);
  const derive = createDeriver();
  const detect = createEventDetector();
  const events = [];
  const rows = [];
  // Slow-frame values (1 Hz) are carried forward into every fast sample, as
  // in live mode. A row without valid_mask holds only slow-frame values.
  let slow = {};
  for (const r of raw) {
    const slowHere = SLOW_KEYS.filter(k => k in r);
    if (slowHere.length) slow = { ...slow, ...Object.fromEntries(slowHere.map(k => [k, r[k]])) };
    if (!('valid_mask' in r)) continue;
    const data = { ...slow, ...r };
    const row = { ...data, ...derive(r.ts, data) };
    events.push(...detect(r.ts, row));
    rows.push(row);
  }
  const ts = rows.map(r => r.ts);
  const columns = ALL_CHANNEL_KEYS.map(k => rows.map(r => r[k] ?? null));
  return { samples: [ts, ...columns], events };
}

export const updateRun = (id, changes) => apiPatch(`/api/runs/${id}`, changes);

export const fetchMarkers = runId => apiGet(`/api/runs/${runId}/markers`);
export const addMarker = (runId, label, ts) => apiPost(`/api/runs/${runId}/markers`, { label, ts });
export const updateMarker = (runId, id, changes) => apiPatch(`/api/runs/${runId}/markers/${id}`, changes);
export const deleteMarker = (runId, id) => apiDelete(`/api/runs/${runId}/markers/${id}`);

// The metadata of the last edited run pre-fills the next recording, since
// driver, mode and setup rarely change between consecutive runs.
const LAST_META_KEY = 'telemetry-last-run-meta';

export function lastMeta() {
  try {
    return JSON.parse(localStorage.getItem(LAST_META_KEY)) ?? {};
  } catch {
    return {};
  }
}

export function saveLastMeta(run) {
  const meta = Object.fromEntries(META_FIELDS.map(f => [f.key, run[f.key] ?? '']));
  try { localStorage.setItem(LAST_META_KEY, JSON.stringify(meta)); } catch {}
}

// Binary search for the index of the sample nearest to ts in a sorted array.
export function nearestIdx(arr, ts) {
  if (!arr?.length) return -1;
  if (ts <= arr[0]) return 0;
  if (ts >= arr[arr.length - 1]) return arr.length - 1;
  let lo = 0, hi = arr.length - 1;
  while (lo < hi - 1) {
    const mid = (lo + hi) >> 1;
    arr[mid] <= ts ? (lo = mid) : (hi = mid);
  }
  return ts - arr[lo] <= arr[hi] - ts ? lo : hi;
}

// Seconds since the start of the run, as m:ss.s.
export function fmtRel(sec) {
  if (sec == null || !isFinite(sec)) return '—';
  const sign = sec < 0 ? '-' : '';
  const s = Math.abs(sec);
  const m = Math.floor(s / 60);
  return `${sign}${m}:${(s - m * 60).toFixed(1).padStart(4, '0')}`;
}

export function fmtValue(v) {
  if (v == null) return '—';
  if (Number.isInteger(v)) return String(v);
  return Math.abs(v) >= 100 ? v.toFixed(1) : v.toFixed(2);
}

export function runDuration(run) {
  if (!run.ended_at) return 'ongoing';
  const s = Math.round(run.ended_at - run.started_at);
  return `${Math.floor(s / 60)}m ${s % 60}s`;
}
