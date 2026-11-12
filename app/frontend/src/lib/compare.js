// Aligning several runs on a common time axis for the Compare tab.
import { ALL_CHANNEL_KEYS } from './channels.js';

/**
 * Finds the timestamp each run is aligned on (its t = 0).
 * align.mode: 'start' | 'marker' (align.label) | 'threshold' (align.channel >= align.value)
 * @returns {{offset: number, found: boolean}}  found is false when the run had
 *          no matching marker/sample and was aligned on its start instead.
 */
export function alignmentOffset(samples, markers, align) {
  const ts = samples[0];
  const start = ts[0];
  if (align.mode === 'marker') {
    const m = markers.find(m => m.label === align.label);
    return m ? { offset: m.ts, found: true } : { offset: start, found: false };
  }
  if (align.mode === 'threshold') {
    const col = samples[ALL_CHANNEL_KEYS.indexOf(align.channel) + 1];
    const i = col ? col.findIndex(v => v != null && v >= align.value) : -1;
    return i >= 0 ? { offset: ts[i], found: true } : { offset: start, found: false };
  }
  return { offset: start, found: true };
}

/**
 * Builds uPlot data for overlaying runs: one shared x axis (seconds from each
 * run's offset) and, per channel, one value array per run with nulls where
 * that run has no sample.
 * @param {{samples: any[][], offset: number}[]} entries
 * @param {string[]} channels
 * @returns {{xs: number[], rel: number[][], byChannel: Record<string, (number|null)[][]>}}
 */
export function buildOverlay(entries, channels) {
  // Rounded to the ms so runs sampled at the same relative time share an x.
  const rel = entries.map(e => e.samples[0].map(t => Math.round((t - e.offset) * 1000) / 1000));
  const xs = [...new Set(rel.flat())].sort((a, b) => a - b);
  const index = new Map(xs.map((t, i) => [t, i]));

  const byChannel = {};
  for (const key of channels) {
    const ci = ALL_CHANNEL_KEYS.indexOf(key) + 1;
    byChannel[key] = entries.map((e, ri) => {
      const out = new Array(xs.length).fill(null);
      const col = e.samples[ci];
      if (col) rel[ri].forEach((t, j) => { out[index.get(t)] = col[j]; });
      return out;
    });
  }
  return { xs, rel, byChannel };
}

// min / max / mean of col over the samples whose relative time is in [lo, hi].
export function rangeStats(relTimes, col, lo = -Infinity, hi = Infinity) {
  let min = Infinity, max = -Infinity, sum = 0, n = 0;
  for (let i = 0; i < relTimes.length; i++) {
    const t = relTimes[i];
    const v = col?.[i];
    if (t < lo || t > hi || v == null) continue;
    if (v < min) min = v;
    if (v > max) max = v;
    sum += v;
    n++;
  }
  return n ? { min, max, avg: sum / n } : null;
}
