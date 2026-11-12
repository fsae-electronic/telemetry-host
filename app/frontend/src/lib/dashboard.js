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
const DRIVER_STATS = ['motor_velocity', 'target_torque', 'driver_temp', 'motor_temp', 'driver_dc_voltage', 'heartbeat_state'];

const DEFAULT_PANELS = [
  {
    id: 'p-tps',
    type: 'timeseries',
    title: 'Pedal',
    channels: ['tps1', 'tps2'],
    col: 1, row: 1, cols: 3, rows: 4,
  },
  // Separate from the pedal: torque is in ‰ (0–1000), the pedal in % (0–100).
  {
    id: 'p-torque',
    type: 'timeseries',
    title: 'Torque',
    channels: ['torque_request', 'torque_cmd_l', 'torque_cmd_r'],
    col: 4, row: 1, cols: 3, rows: 4,
  },
  {
    id: 'p-brakes',
    type: 'timeseries',
    title: 'Brakes',
    channels: ['front_brake', 'rear_brake'],
    col: 7, row: 1, cols: 6, rows: 4,
  },
  {
    id: 'p-wheels',
    type: 'timeseries',
    title: 'Wheel Speeds',
    channels: ['wheel_speed_fl', 'wheel_speed_fr'],
    col: 1, row: 5, cols: 6, rows: 4,
  },
  {
    id: 'p-currents',
    type: 'timeseries',
    title: 'Currents',
    channels: ['ac_current_1', 'ac_current_2', 'dc_current_1', 'dc_current_2'],
    col: 7, row: 5, cols: 6, rows: 4,
  },
  {
    id: 'p-driver0',
    type: 'stat',
    title: 'Driver L',
    channels: DRIVER_STATS.map(f => 'drv0_' + f),
    col: 1, row: 9, cols: 6, rows: 3,
  },
  {
    id: 'p-driver1',
    type: 'stat',
    title: 'Driver R',
    channels: DRIVER_STATS.map(f => 'drv1_' + f),
    col: 7, row: 9, cols: 6, rows: 3,
  },
  {
    id: 'p-steering',
    type: 'timeseries',
    title: 'Steering Angle',
    channels: ['steering_angle'],
    col: 1, row: 12, cols: 6, rows: 3,
  },
  {
    id: 'p-status',
    type: 'stat',
    title: 'Frame / Driver Status',
    channels: ['valid_mask', 'calibration_type', 'drv0_error', 'drv1_error', 'drv0_warnings', 'drv1_warnings'],
    col: 7, row: 12, cols: 6, rows: 3,
  },
];

// Bump when channel keys change so stale saved layouts are ignored.
const STORAGE_KEY = 'telemetry-dashboard-v2';

function load() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) return JSON.parse(raw);
  } catch {}
  return DEFAULT_PANELS;
}

function createDashboard() {
  const { subscribe, set, update } = writable(load());

  function save(panels) {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(panels));
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
