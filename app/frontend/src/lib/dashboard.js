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
    id: 'p-brakes',
    type: 'timeseries',
    title: 'Brake Sensors',
    channels: ['s_00', 's_01'],
    col: 1, row: 1, cols: 6, rows: 4,
  },
  {
    id: 'p-battery',
    type: 'gauge',
    title: 'Battery Level',
    channels: ['s_02'],
    col: 7, row: 1, cols: 3, rows: 4,
  },
  {
    id: 'p-steering',
    type: 'gauge',
    title: 'Steering Angle',
    channels: ['s_03'],
    col: 10, row: 1, cols: 3, rows: 4,
  },
  {
    id: 'p-accel',
    type: 'bargauge',
    title: 'Accelerator Pedal',
    channels: ['s_04', 's_05'],
    col: 1, row: 5, cols: 3, rows: 3,
  },
  {
    id: 'p-brakeped',
    type: 'bargauge',
    title: 'Brake Pedal',
    channels: ['s_06', 's_07'],
    col: 4, row: 5, cols: 3, rows: 3,
  },
  {
    id: 'p-currents',
    type: 'timeseries',
    title: 'Motor Currents',
    channels: ['s_08', 's_09', 's_10', 's_11'],
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
