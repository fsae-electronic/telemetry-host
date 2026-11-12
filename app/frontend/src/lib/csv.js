// CSV export of a run (or part of it), built in the browser from the loaded
// samples so it includes the derived channels.
import { ALL_CHANNEL_KEYS } from './channels.js';
import { nearestIdx } from './runs.js';

/**
 * @param {object} o
 * @param {any[][]} o.samples  uPlot-format run data ordered by ALL_CHANNEL_KEYS
 * @param {string[]} o.keys    channels to export, in column order
 * @param {{ts: number, label: string}[]} o.markers
 * @param {number} [o.from]    first timestamp to include (inclusive)
 * @param {number} [o.to]      last timestamp to include (inclusive)
 * @param {boolean} [o.excel]  ';' separator and ',' decimals, for Excel set to Spanish
 * @returns {{text: string, rows: number}}
 */
export function buildCsv({ samples, keys, markers = [], from = -Infinity, to = Infinity, excel = false }) {
  const sep = excel ? ';' : ',';
  const ts = samples[0];
  const t0 = ts[0];
  const cols = keys.map(k => samples[ALL_CHANNEL_KEYS.indexOf(k) + 1]);

  // Each marker goes on the row of the sample nearest to it.
  const markerAt = new Map();
  for (const m of markers) {
    const i = nearestIdx(ts, m.ts);
    markerAt.set(i, markerAt.has(i) ? `${markerAt.get(i)} | ${m.label}` : m.label);
  }

  const num = v => {
    if (v == null || !isFinite(v)) return '';
    const s = String(Number.isInteger(v) ? v : Number(v.toFixed(6)));
    return excel ? s.replace('.', ',') : s;
  };
  const text = s => (/[",;\n\r]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s);

  const lines = [['time_s', 'timestamp', 'marker', ...keys].join(sep)];
  for (let i = 0; i < ts.length; i++) {
    if (ts[i] < from || ts[i] > to) continue;
    const row = [num(ts[i] - t0), num(ts[i]), text(markerAt.get(i) ?? '')];
    for (const col of cols) row.push(num(col?.[i]));
    lines.push(row.join(sep));
  }
  return { text: lines.join('\r\n') + '\r\n', rows: lines.length - 1 };
}

export function downloadText(filename, text, { bom = false } = {}) {
  // Excel only detects UTF-8 (accents in labels) when the file starts with a BOM.
  const blob = new Blob([bom ? '﻿' + text : text], { type: 'text/csv;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

export function safeFilename(name) {
  return name.replace(/[\\/:*?"<>|]+/g, '-').replace(/\s+/g, '_').slice(0, 80) || 'run';
}
