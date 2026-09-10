import { writable } from 'svelte/store';

// Panel types
export const PANEL_TYPES = {
  timeseries: { label: 'Time Series',  icon: '📈' },
  gauge:      { label: 'Gauge',         icon: '🕹️' },
  stat:       { label: 'Stat',          icon: '🔢' },
  bargauge:   { label: 'Bar Gauge',     icon: '▬' },
  scatter:    { label: 'XY Scatter',    icon: '✦' },
};

// Grid is 12 columns wide; row height is 80px.
const DEFAULT_PANELS = [
  {
    id: 'p-tps',
    type: 'timeseries',
    title: 'TPS',
    channels: ['tps1', 'tps2'],
    col: 1, row: 1, cols: 4, rows: 4,
  },
  {
    id: 'p-steering',
    type: 'gauge',
    title: 'Steering Angle',
    channels: ['steering_angle'],
    col: 5, row: 1, cols: 3, rows: 4,
  },
  {
    id: 'p-front-brake',
    type: 'stat',
    title: 'Front Brake',
    channels: ['front_brake'],
    col: 8, row: 1, cols: 2, rows: 4,
  },
  {
    id: 'p-rear-brake',
    type: 'stat',
    title: 'Rear Brake',
    channels: ['rear_brake'],
    col: 10, row: 1, cols: 3, rows: 4,
  },
  {
    id: 'p-wheel-speed',
    type: 'timeseries',
    title: 'Front Wheel Speed',
    channels: ['wheel_speed_fl', 'wheel_speed_fr'],
    col: 1, row: 5, cols: 6, rows: 3,
  },
  {
    id: 'p-currents',
    type: 'timeseries',
    title: 'ADC Currents',
    channels: ['ac_current_1', 'ac_current_2', 'dc_current_1', 'dc_current_2'],
    col: 7, row: 5, cols: 6, rows: 3,
  },
];

function load() {
  try {
    const raw = localStorage.getItem('telemetry-dashboard');
    if (raw) return JSON.parse(raw);
  } catch {}
  return DEFAULT_PANELS;
}

function createDashboard() {
  const { subscribe, set, update } = writable(load());

  function save(panels) {
    localStorage.setItem('telemetry-dashboard', JSON.stringify(panels));
  }

  return {
    subscribe,
    addPanel(panel) {
      update(panels => {
        const next = [...panels, panel];
        save(next);
        return next;
      });
    },
    updatePanel(id, changes) {
      update(panels => {
        const next = panels.map(p => p.id === id ? { ...p, ...changes } : p);
        save(next);
        return next;
      });
    },
    removePanel(id) {
      update(panels => {
        const next = panels.filter(p => p.id !== id);
        save(next);
        return next;
      });
    },
    movePanel(id, col, row) {
      update(panels => {
        const next = panels.map(p => p.id === id ? { ...p, col, row } : p);
        save(next);
        return next;
      });
    },
    reset() {
      set(DEFAULT_PANELS);
      save(DEFAULT_PANELS);
    },
  };
}

export const dashboard = createDashboard();
