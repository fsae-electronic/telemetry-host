// Decoders for the diagnostic fields of the v3 frame (bit fields of the main
// ECU, see "Frame de telemetría v3") and the CANopen / CiA 402 words
// reported by each motor driver.

export const DRIVER_LABELS = ['Driver L', 'Driver R'];

// valid_mask / fresh_mask bits. valid: the CAN message has been received at
// least once since the main ECU booted; fresh: received in the last 200 ms
// (1000 ms for TXPDO1, which the drivers send every 500 ms).
export const VALID_BITS = [
  { bit: 0, group: 'Sensors', name: 'TPS', title: '0x501' },
  { bit: 1, group: 'Sensors', name: 'Front', title: '0x502: front wheel speeds, steering, front brake' },
  { bit: 2, group: 'Sensors', name: 'Currents', title: '0x503' },
  { bit: 3, group: 'Sensors', name: 'Rear', title: '0x504: rear brake' },
  { bit: 4, group: 'Sensors', name: 'Calibration', title: '0x600' },
  ...[0, 1].flatMap(n =>
    ['RXPDO1', 'TXPDO1', 'TXPDO2', 'TXPDO3', 'Heartbeat'].map((name, i) => ({
      bit: 5 + 5 * n + i, group: DRIVER_LABELS[n], name,
    }))
  ),
  { bit: 15, group: 'Vehicle', name: 'BMS' },
  { bit: 16, group: 'Vehicle', name: 'Rear wheels', title: 'Derived from both drivers\' TXPDO2' },
  { bit: 17, group: 'Vehicle', name: 'IMU' },
  { bit: 18, group: 'Vehicle', name: 'Coolant' },
  { bit: 19, group: 'Vehicle', name: 'LV' },
  { bit: 20, group: 'Vehicle', name: 'Tractive system' },
];

export function validBitNames(mask) {
  return VALID_BITS.filter(b => mask & (1 << b.bit)).map(b => `${b.group} ${b.name}`);
}

// ecu_state bits 2–10 (bits 0–1 are the mode, see ecuMode).
export const ECU_STATE_BITS = [
  [2, 'Drive enabled'],
  [3, 'Ready to drive'],
  [4, 'Traction control on'],
  [5, 'TC cutting L'],
  [6, 'TC cutting R'],
  [7, 'APPS implausible'],
  [8, 'Brake/throttle (EV.4.7)'],
  [9, 'Brake pressed'],
  [10, 'Safe mode'],
];

export function ecuMode(state) {
  return ['Normal', 'Race', 'Mode 2', 'Mode 3'][(state ?? 0) & 3];
}

export const TORQUE_LIMIT_BITS = [
  [0, 'Drive off'],
  [1, 'APPS'],
  [2, 'EV.4.7'],
  [3, 'Traction control'],
  [4, 'Power (race map)'],
  [5, 'BMS current'],
  [6, 'Motor temp'],
  [7, 'Driver temp'],
  [8, 'Cell temp'],
  [9, 'Low cell voltage'],
  [10, 'Speed limit'],
  [11, 'Driver not enabled'],
];

export const TS_STATE_BITS = [
  [0, 'SDC closed'],
  [1, 'AIR+'],
  [2, 'AIR−'],
  [3, 'Precharge relay'],
  [4, 'Precharge done'],
  [5, 'IMD ok'],
  [6, 'AMS ok'],
  [7, 'BSPD ok'],
  [8, 'TS active'],
  [9, 'TSMS on'],
];

// active_faults (slow frame), as defined in telemetry.c.
export const ACTIVE_FAULT_BITS = [
  [0, 'TPS timeout (0x501)'],
  [1, 'Front sensors timeout (0x502)'],
  [2, 'Rear sensors timeout (0x504)'],
  [3, 'Sensor implausibility (0x401)'],
  [4, 'Brake/throttle plausibility (EV.4.7)'],
];

const RESET_CAUSES = ['Unknown', 'Power-on', 'Watchdog', 'External reset', 'Software', 'Oscillator failure', 'CPU reset (ESM)', 'Brown-out'];

export function resetCause(v) {
  return RESET_CAUSES[v] ?? `Unknown (${v})`;
}

// Names of the bits set in v, from a [bit, name] table.
export function bitNames(v, table) {
  return table.filter(([bit]) => (v ?? 0) & (1 << bit)).map(([, name]) => name);
}

// "0.3.0 · 0885e466 · dirty" from the slow frame, or null before it arrives.
export function firmwareLabel(v) {
  if (v.git_hash == null && v.fw_minor == null) return null;
  const hash = ((v.git_hash ?? 0) >>> 0).toString(16).padStart(8, '0');
  return `${v.fw_major ?? 0}.${v.fw_minor ?? 0}.${v.fw_patch ?? 0} · ${hash}${v.fw_dirty ? ' · dirty' : ''}`;
}

// CAN error counter level (TEC/REC): active below 96, warning 96–127,
// error passive from 128.
export function canErrorLevel(count) {
  if (count >= 128) return 'error';
  if (count >= 96) return 'warn';
  return 'ok';
}

// True once any CAN message from driver n has been received.
export function driverSeen(mask, n) {
  return ((mask >> (5 + 5 * n)) & 0x1F) !== 0;
}

// CiA 402 state machine, decoded from the status word: [mask, value, name, level].
const CIA402_STATES = [
  [0x4F, 0x00, 'Not ready to switch on', 'idle'],
  [0x4F, 0x40, 'Switch on disabled',     'idle'],
  [0x6F, 0x21, 'Ready to switch on',     'idle'],
  [0x6F, 0x23, 'Switched on',            'idle'],
  [0x6F, 0x27, 'Operation enabled',      'ok'],
  [0x6F, 0x07, 'Quick stop active',      'warn'],
  [0x4F, 0x0F, 'Fault reaction active',  'error'],
  [0x4F, 0x08, 'Fault',                  'error'],
];

export function cia402State(statusWord) {
  const sw = statusWord ?? 0;
  for (const [mask, value, name, level] of CIA402_STATES) {
    if ((sw & mask) === value) return { name, level };
  }
  return { name: 'Unknown', level: 'warn' };
}

const STATUS_FLAGS = [
  [4, 'Voltage enabled'],
  [7, 'Warning'],
  [9, 'Remote'],
  [10, 'Target reached'],
  [11, 'Internal limit'],
];

export function statusFlags(statusWord) {
  return STATUS_FLAGS.filter(([bit]) => (statusWord ?? 0) & (1 << bit)).map(([, name]) => name);
}

// CANopen NMT state carried by the heartbeat.
const NMT_STATES = { 0: 'Boot-up', 4: 'Stopped', 5: 'Operational', 127: 'Pre-operational' };

export function nmtState(v) {
  return NMT_STATES[v] ?? `Unknown (${v})`;
}

export function isNmtState(v) {
  return v in NMT_STATES;
}

// CANopen emergency error code classes (CiA 301). The exact meaning of each
// code is in the driver manual; this only gives the category.
export function errorClass(code) {
  if (!code) return 'No error';
  const hi = code >> 8;
  if (hi === 0xFF) return 'Device specific';
  if (hi === 0x81) return 'Communication';
  if (hi === 0x82) return 'Protocol';
  const classes = {
    0x1: 'Generic', 0x2: 'Current', 0x3: 'Voltage', 0x4: 'Temperature',
    0x5: 'Device hardware', 0x6: 'Device software', 0x7: 'Additional modules',
    0x8: 'Monitoring', 0x9: 'External', 0xF: 'Additional functions',
  };
  return classes[code >> 12] ?? 'Unknown';
}

export function setBits(v) {
  const out = [];
  for (let b = 0; b < 16; b++) if ((v ?? 0) & (1 << b)) out.push(b);
  return out;
}

export function hex(v, digits = 4) {
  return '0x' + ((v ?? 0) >>> 0).toString(16).toUpperCase().padStart(digits, '0');
}
