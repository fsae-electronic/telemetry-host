import { writable } from 'svelte/store';
import { ALL_CHANNEL_KEYS } from './channels.js';
import { createDeriver } from './derived.js';
import { createEventDetector } from './events.js';

export const status = writable({
  connected: false,
  port: '',
  recording: false,
  active_run_id: 0,
  wsReady: false,
});

export const latestPacket = writable(null);

// Serial link counters pushed by the server once per second (see api.BroadcastLinkStats).
export const link = writable(null);

// Events detected on the live stream (see events.js), oldest first.
const MAX_EVENTS = 500;
export const liveEvents = writable([]);

// Markers added to the run being recorded, for drawing on live charts.
export const liveMarkers = writable([]);

// Derived channels and events keep state per session: a new session starts
// on (re)connect and when a recording starts, so energy counts from there.
let derive = createDeriver();
let detect = createEventDetector();
let lastConnected = false;
let lastRunId = 0;

// Latest slow-frame (1 Hz) values, merged into every fast packet so each
// sample carries the whole state of the car.
let slowValues = {};

function onStatus(s) {
  if (s.connected && !lastConnected) {
    derive = createDeriver();
    detect = createEventDetector();
    slowValues = {};
  }
  if (s.active_run_id && s.active_run_id !== lastRunId) {
    derive = createDeriver();
    liveMarkers.set([]);
  }
  lastConnected = s.connected;
  lastRunId = s.active_run_id;
}

export function clearLiveEvents() {
  liveEvents.set([]);
}

// Ring buffer: last MAX_POINTS packets
export const MAX_POINTS = 600;

// latest value per channel — for gauges, stats, etc.
export const liveValues = writable({});

function makeBuffer() {
  const ts = new Float64Array(MAX_POINTS);
  // keyed by channel key → Float64Array
  const channels = {};
  for (const key of ALL_CHANNEL_KEYS) {
    channels[key] = new Float64Array(MAX_POINTS);
  }
  let head = 0;
  let count = 0;
  return { ts, channels, head, count };
}

function createLiveBufferStore() {
  let buf = makeBuffer();
  const { subscribe, set } = writable(buf);

  function push(packet) {
    const idx = buf.head;
    buf.ts[idx] = packet.ts;
    const vals = {};
    for (const key of ALL_CHANNEL_KEYS) {
      const v = packet.data[key] ?? 0;
      buf.channels[key][idx] = v;
      vals[key] = v;
    }
    buf.head = (buf.head + 1) % MAX_POINTS;
    buf.count = Math.min(buf.count + 1, MAX_POINTS);
    set({ ...buf });
    liveValues.set(vals);
  }

  function reset() {
    buf = makeBuffer();
    set(buf);
  }

  return { subscribe, push, reset };
}

export const liveBuffer = createLiveBufferStore();

let ws = null;
let reconnectTimer = null;
let reconnectDelay = 1000;

function connect() {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const url = `${proto}//${location.host}/ws`;

  ws = new WebSocket(url);

  ws.onopen = () => {
    status.update(s => ({ ...s, wsReady: true }));
    reconnectDelay = 1000;
  };

  ws.onmessage = (e) => {
    let msg;
    try { msg = JSON.parse(e.data); } catch { return; }

    if (msg.type === 'packet' && msg.kind === 'slow') {
      slowValues = msg.data;
    } else if (msg.type === 'packet') {
      const data = { ...slowValues, ...msg.data };
      msg.data = { ...data, ...derive(msg.ts, data) };
      const events = detect(msg.ts, msg.data);
      if (events.length) liveEvents.update(l => [...l, ...events].slice(-MAX_EVENTS));
      latestPacket.set(msg);
      liveBuffer.push(msg);
    } else if (msg.type === 'status') {
      onStatus(msg.data);
      status.update(s => ({ ...s, ...msg.data }));
    } else if (msg.type === 'link') {
      link.set(msg.data);
    }
  };

  ws.onclose = () => {
    status.update(s => ({ ...s, wsReady: false }));
    reconnectTimer = setTimeout(() => {
      reconnectDelay = Math.min(reconnectDelay * 1.5, 15000);
      connect();
    }, reconnectDelay);
  };

  ws.onerror = () => ws.close();
}

connect();

export async function apiGet(path) {
  const r = await fetch(path);
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export async function apiPost(path, body) {
  const r = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body ?? {}),
  });
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export async function apiPatch(path, body) {
  const r = await fetch(path, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body ?? {}),
  });
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export async function apiDelete(path) {
  const r = await fetch(path, { method: 'DELETE' });
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}
