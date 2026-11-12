// Channels decoded from the main ECU's v3 frames (see internal/serial/frame.go),
// plus the channels the host calculates from them (see derived.js). Values
// arrive in physical units; steering and the currents are still the raw
// values the ECU reads from CAN until it sends them calibrated.
// `slow: true` marks channels of the 1 Hz slow frame.
import { DERIVED_CHANNELS } from './derived.js';

const SENSORS = {
  tps1:             { name: 'TPS 1',            group: 'tps',      unit: '%',   color: '#FFB74D' },
  tps2:             { name: 'TPS 2',            group: 'tps',      unit: '%',   color: '#FFCC80' },
  // Torque the main ECU requests, ‰ of rated torque (1000 = rated).
  torque_request:   { name: 'Torque Request',   group: 'tps',      unit: '‰',   color: '#B39DDB' },
  torque_cmd_l:     { name: 'Torque L',         group: 'tps',      unit: '‰',   color: '#CE93D8' },
  torque_cmd_r:     { name: 'Torque R',         group: 'tps',      unit: '‰',   color: '#F48FB1' },
  wheel_speed_fl:   { name: 'Wheel Speed FL',   group: 'wheels',   unit: 'rpm', color: '#4FC3F7' },
  wheel_speed_fr:   { name: 'Wheel Speed FR',   group: 'wheels',   unit: 'rpm', color: '#81D4FA' },
  // The ECU derives the rear ones from motor velocity / gear ratio.
  wheel_speed_rl:   { name: 'Wheel Speed RL',   group: 'wheels',   unit: 'rpm', color: '#29B6F6' },
  wheel_speed_rr:   { name: 'Wheel Speed RR',   group: 'wheels',   unit: 'rpm', color: '#B3E5FC' },
  steering_angle:   { name: 'Steering Angle',   group: 'steering', unit: 'raw', color: '#64B5F6' },
  front_brake:      { name: 'Front Brake',      group: 'brakes',   unit: 'PSI', color: '#E57373' },
  rear_brake:       { name: 'Rear Brake',       group: 'brakes',   unit: 'PSI', color: '#EF9A9A' },
  // 1 = left, 2 = right
  dc_current_1:     { name: 'DC Current L',     group: 'currents', unit: 'raw', color: '#26C6DA' },
  dc_current_2:     { name: 'DC Current R',     group: 'currents', unit: 'raw', color: '#80DEEA' },
  ac_current_1:     { name: 'AC Current L',     group: 'currents', unit: 'raw', color: '#4DB6AC' },
  ac_current_2:     { name: 'AC Current R',     group: 'currents', unit: 'raw', color: '#80CBC4' },

  pack_voltage:      { name: 'Pack Voltage',     group: 'bms', unit: 'V',  color: '#FF8A65' },
  pack_current:      { name: 'Pack Current',     group: 'bms', unit: 'A',  color: '#FFAB91' },
  soc:               { name: 'State of Charge',  group: 'bms', unit: '%',  color: '#AED581' },
  cell_v_min:        { name: 'Cell V Min',       group: 'bms', unit: 'V',  color: '#FFD54F' },
  cell_v_max:        { name: 'Cell V Max',       group: 'bms', unit: 'V',  color: '#FFE082' },
  cell_temp_max:     { name: 'Cell Temp Max',    group: 'bms', unit: '°C', color: '#FF7043' },
  bms_current_limit: { name: 'BMS Current Limit', group: 'bms', unit: 'A', color: '#BCAAA4' },
  bms_fault:         { name: 'BMS Faults',       group: 'bms', unit: '',   color: '#E57373' },

  accel_x:      { name: 'Accel X',       group: 'vehicle', unit: 'g',   color: '#9575CD' },
  accel_y:      { name: 'Accel Y',       group: 'vehicle', unit: 'g',   color: '#7986CB' },
  yaw_rate:     { name: 'Yaw Rate',      group: 'vehicle', unit: '°/s', color: '#4DD0E1' },
  coolant_temp: { name: 'Coolant Temp',  group: 'vehicle', unit: '°C',  color: '#4FC3F7' },
  lv_voltage:   { name: 'LV Voltage',    group: 'vehicle', unit: 'V',   color: '#DCE775' },

  ecu_state:           { name: 'ECU State',           group: 'ecu', unit: '', color: '#90A4AE' },
  torque_limit_reason: { name: 'Torque Limit Reason', group: 'ecu', unit: '', color: '#A1887F' },
  ts_state:            { name: 'Tractive System',     group: 'ecu', unit: '', color: '#FF8A80' },
  sdc_open_segment:    { name: 'SDC Open Segment',    group: 'ecu', unit: '', color: '#FF5252' },
  calibration_type:    { name: 'Calibration Type',    group: 'ecu', unit: '', color: '#B0BEC5' },
  valid_mask:          { name: 'Valid Mask',          group: 'ecu', unit: '', color: '#90A4AE' },
  fresh_mask:          { name: 'Fresh Mask',          group: 'ecu', unit: '', color: '#78909C' },
  seq:                 { name: 'Frame Seq',           group: 'ecu', unit: '', color: '#607D8B' },
  ecu_time:            { name: 'ECU Time',            group: 'ecu', unit: 's', color: '#546E7A' },

  fw_major:         { name: 'FW Major',          group: 'ecuslow', unit: '',   color: '#B0BEC5', slow: true },
  fw_minor:         { name: 'FW Minor',          group: 'ecuslow', unit: '',   color: '#B0BEC5', slow: true },
  fw_patch:         { name: 'FW Patch',          group: 'ecuslow', unit: '',   color: '#B0BEC5', slow: true },
  fw_dirty:         { name: 'FW Dirty',          group: 'ecuslow', unit: '',   color: '#B0BEC5', slow: true },
  git_hash:         { name: 'Git Hash',          group: 'ecuslow', unit: '',   color: '#B0BEC5', slow: true },
  uptime_s:         { name: 'Uptime',            group: 'ecuslow', unit: 's',  color: '#90A4AE', slow: true },
  reset_count:      { name: 'Reset Count',       group: 'ecuslow', unit: '',   color: '#90A4AE', slow: true },
  last_reset_cause: { name: 'Last Reset Cause',  group: 'ecuslow', unit: '',   color: '#90A4AE', slow: true },
  cpu_load:         { name: 'CPU Load',          group: 'ecuslow', unit: '%',  color: '#FFB74D', slow: true },
  loop_overruns:    { name: 'Loop Overruns',     group: 'ecuslow', unit: '',   color: '#E57373', slow: true },
  loop_time_max:    { name: 'Loop Time Max',     group: 'ecuslow', unit: 'µs', color: '#FFCC80', slow: true },
  active_faults:    { name: 'Active Faults',     group: 'ecuslow', unit: '',   color: '#EF5350', slow: true },
  can_tx_err:       { name: 'CAN TX Errors',     group: 'ecuslow', unit: '',   color: '#BA68C8', slow: true },
  can_rx_err:       { name: 'CAN RX Errors',     group: 'ecuslow', unit: '',   color: '#CE93D8', slow: true },
  can_busoff_count: { name: 'CAN Bus-off Count', group: 'ecuslow', unit: '',   color: '#AB47BC', slow: true },
  cell_v_min_id:    { name: 'Cell V Min ID',     group: 'ecuslow', unit: '',   color: '#BCAAA4', slow: true },
  cell_v_max_id:    { name: 'Cell V Max ID',     group: 'ecuslow', unit: '',   color: '#BCAAA4', slow: true },
  cell_temp_max_id: { name: 'Cell Temp Max ID',  group: 'ecuslow', unit: '',   color: '#BCAAA4', slow: true },
};

// [field, name, unit, slow]
const DRIVER_FIELDS = [
  ['control_word',        'Control Word',        ''],
  ['status_word',         'Status Word',         ''],
  ['warnings',            'Warnings',            ''],
  ['error',               'Error',               ''],
  ['target_torque',       'Target Torque',       '‰'],
  ['torque_actual',       'Torque Actual',       '‰'],
  ['current_actual',      'Current Actual',      '‰'],
  ['motor_velocity',      'Motor Velocity',      'rpm'],
  ['driver_dc_voltage',   'Driver DC Voltage',   'V'],
  ['driver_temp',         'Driver Temp',         '°C'],
  ['motor_temp',          'Motor Temp',          '°C'],
  ['heartbeat_state',     'Heartbeat State',     ''],
  ['motor_rated_current', 'Motor Rated Current', 'mA', true],
  ['motor_rated_torque',  'Motor Rated Torque',  'Nm', true],
];

const DRIVERS = [
  { prefix: 'drv0_', label: 'Driver L', group: 'driver0', color: '#CE93D8' },
  { prefix: 'drv1_', label: 'Driver R', group: 'driver1', color: '#F48FB1' },
];

function driverChannels() {
  const out = {};
  for (const d of DRIVERS) {
    for (const [field, name, unit, slow] of DRIVER_FIELDS) {
      out[d.prefix + field] = { name: `${d.label} ${name}`, group: d.group, unit, color: d.color, slow: !!slow };
    }
  }
  return out;
}

function derivedChannels() {
  return Object.fromEntries(
    Object.entries(DERIVED_CHANNELS).map(([key, meta]) => [key, { ...meta, group: 'derived' }])
  );
}

export const CHANNELS = { ...SENSORS, ...driverChannels(), ...derivedChannels() };

// Keys sent by the car, i.e. everything that is not calculated on the host.
export const RAW_CHANNEL_KEYS = Object.keys(CHANNELS).filter(k => CHANNELS[k].group !== 'derived');

// Keys of the 1 Hz slow frame; their latest values are merged into every
// fast sample (live in ws.js, saved runs in runs.js).
export const SLOW_KEYS = Object.keys(CHANNELS).filter(k => CHANNELS[k].slow);

const GROUP_LABELS = {
  tps:      'Pedal / Torque',
  wheels:   'Wheel Speeds',
  steering: 'Steering',
  brakes:   'Brakes',
  currents: 'Currents',
  bms:      'Accumulator',
  vehicle:  'IMU / Cooling / LV',
  ecu:      'Main ECU',
  ecuslow:  'Main ECU (1 Hz)',
  driver0:  'Driver L',
  driver1:  'Driver R',
  derived:  'Calculated',
};

export const GROUPS = Object.fromEntries(
  Object.entries(GROUP_LABELS).map(([key, label]) => [
    key,
    { label, channels: Object.keys(CHANNELS).filter(k => CHANNELS[k].group === key) },
  ])
);

export const ALL_CHANNEL_KEYS = Object.keys(CHANNELS);
