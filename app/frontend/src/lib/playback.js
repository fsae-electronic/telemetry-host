import { ALL_CHANNEL_KEYS } from './channels.js';

/**
 * Given the full run samples (uPlot format: [ts[], ch0[], ..., chN[]], ordered by ALL_CHANNEL_KEYS)
 * and a sample index, return a key→value map for point-in-time panels.
 */
export function valuesAtIndex(samples, idx) {
  if (!samples?.[0]?.length) return {};
  const i = Math.max(0, Math.min(idx, samples[0].length - 1));
  return Object.fromEntries(
    ALL_CHANNEL_KEYS.map((key, ci) => [key, samples[ci + 1]?.[i] ?? 0])
  );
}

/**
 * Slice the full samples array down to just the channels requested,
 * in the order requested. Returns uPlot-format data.
 */
export function samplesForChannels(samples, channelKeys) {
  if (!samples) return null;
  const ts = samples[0];
  const series = channelKeys.map(key => {
    const ci = ALL_CHANNEL_KEYS.indexOf(key);
    return ci >= 0 ? samples[ci + 1] : new Array(ts.length).fill(null);
  });
  return [ts, ...series];
}
