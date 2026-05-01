export const CHANNELS = {
  s_00: { name: 'Brake Sensor 1',    group: 'brakes',    unit: 'bar', color: '#E57373' },
  s_01: { name: 'Brake Sensor 2',    group: 'brakes',    unit: 'bar', color: '#EF9A9A' },
  s_02: { name: 'Battery Level',     group: 'battery',   unit: '%',   color: '#81C784' },
  s_03: { name: 'Steering Angle',    group: 'steering',  unit: '°',   color: '#4FC3F7' },
  s_04: { name: 'Accel Pedal 1',     group: 'accel',     unit: '%',   color: '#FFB74D' },
  s_05: { name: 'Accel Pedal 2',     group: 'accel',     unit: '%',   color: '#FFCC80' },
  s_06: { name: 'Brake Pedal 1',     group: 'brakeped',  unit: '%',   color: '#CE93D8' },
  s_07: { name: 'Brake Pedal 2',     group: 'brakeped',  unit: '%',   color: '#E1BEE7' },
  s_08: { name: 'Motor Current FL',  group: 'currents',  unit: 'A',   color: '#4DB6AC' },
  s_09: { name: 'Motor Current FR',  group: 'currents',  unit: 'A',   color: '#80CBC4' },
  s_10: { name: 'Motor Current RL',  group: 'currents',  unit: 'A',   color: '#26C6DA' },
  s_11: { name: 'Motor Current RR',  group: 'currents',  unit: 'A',   color: '#80DEEA' },
};

export const GROUPS = {
  brakes:   { label: 'Brake Sensors',   channels: ['s_00', 's_01'] },
  battery:  { label: 'Battery',          channels: ['s_02'] },
  steering: { label: 'Steering',         channels: ['s_03'] },
  accel:    { label: 'Accelerator',      channels: ['s_04', 's_05'] },
  brakeped: { label: 'Brake Pedal',      channels: ['s_06', 's_07'] },
  currents: { label: 'Motor Currents',   channels: ['s_08', 's_09', 's_10', 's_11'] },
};

export const ALL_CHANNEL_KEYS = Object.keys(CHANNELS);
