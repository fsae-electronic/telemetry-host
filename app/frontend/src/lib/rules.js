export const V_BUS_NOMINAL = 400; // volts — used to estimate motor power
export const MOTOR_POWER_LIMIT_W = 80_000;

export const RULES = [
  {
    id: 'apps_implausibility',
    title: 'APPS Implausibility',
    rule_ref: 'T.4.2.4',
    description: 'Accelerator pedal sensors disagree by more than 10% for over 100 ms.',
    sensors: ['s_04', 's_05'],
    severity: 'critical',
    timing_ms: 100,
    latch: false,
    latch_clear_fn: null,
    condition: (v) => Math.abs((v.s_04 ?? 0) - (v.s_05 ?? 0)) > 10,
  },
  {
    id: 'apps_brake_plausibility',
    title: 'APPS + Brake Plausibility',
    rule_ref: 'EV.4.7.1',
    description: 'Brake is engaged while accelerator is pressed beyond 25%. Motor output must be cut.',
    sensors: ['s_04', 's_05', 's_06', 's_07'],
    severity: 'critical',
    timing_ms: 0,
    latch: true,
    latch_clear_fn: (v) => (v.s_04 ?? 0) < 5 && (v.s_05 ?? 0) < 5,
    condition: (v) =>
      ((v.s_06 ?? 0) > 10 || (v.s_07 ?? 0) > 10) &&
      ((v.s_04 ?? 0) > 25 || (v.s_05 ?? 0) > 25),
  },
  {
    id: 'bspd_activation',
    title: 'BSPD Activation',
    rule_ref: 'EV.7.7.2',
    description: 'Hard braking while motor is drawing significant current for over 500 ms.',
    sensors: ['s_06', 's_07', 's_08', 's_09', 's_10', 's_11'],
    severity: 'critical',
    timing_ms: 500,
    latch: false,
    latch_clear_fn: null,
    condition: (v) =>
      ((v.s_06 ?? 0) > 25 || (v.s_07 ?? 0) > 25) &&
      (Math.abs(v.s_08 ?? 0) > 30 ||
       Math.abs(v.s_09 ?? 0) > 30 ||
       Math.abs(v.s_10 ?? 0) > 30 ||
       Math.abs(v.s_11 ?? 0) > 30),
  },
  {
    id: 'regen_while_braking',
    title: 'Regen While Braking',
    rule_ref: 'EV.3.3.3',
    description: 'Regenerative braking detected while brake pedal is engaged.',
    sensors: ['s_06', 's_07', 's_08', 's_09', 's_10', 's_11'],
    severity: 'warning',
    timing_ms: 0,
    latch: false,
    latch_clear_fn: null,
    condition: (v) =>
      ((v.s_06 ?? 0) > 10 || (v.s_07 ?? 0) > 10) &&
      ((v.s_08 ?? 0) < -2 || (v.s_09 ?? 0) < -2 ||
       (v.s_10 ?? 0) < -2 || (v.s_11 ?? 0) < -2),
  },
  {
    id: 'motor_power_limit',
    title: 'Motor Power Limit Exceeded',
    rule_ref: 'EV.3.3.1 / EV.3.4.1',
    description: `Estimated motor power exceeds 80 kW (using ${V_BUS_NOMINAL} V bus) for over 100 ms.`,
    sensors: ['s_08', 's_09', 's_10', 's_11'],
    severity: 'critical',
    timing_ms: 100,
    latch: false,
    latch_clear_fn: null,
    condition: (v) => {
      const totalCurrent =
        Math.abs(v.s_08 ?? 0) + Math.abs(v.s_09 ?? 0) +
        Math.abs(v.s_10 ?? 0) + Math.abs(v.s_11 ?? 0);
      return totalCurrent * V_BUS_NOMINAL > MOTOR_POWER_LIMIT_W;
    },
  },
  {
    id: 'brake_sensor_disagreement',
    title: 'Brake Sensor Disagreement',
    rule_ref: '— (dual-sensor)',
    description: 'Redundant brake pressure sensors differ by more than 0.5 bar for over 200 ms.',
    sensors: ['s_00', 's_01'],
    severity: 'warning',
    timing_ms: 200,
    latch: false,
    latch_clear_fn: null,
    condition: (v) => Math.abs((v.s_00 ?? 0) - (v.s_01 ?? 0)) > 0.5,
  },
  {
    id: 'brake_pedal_disagreement',
    title: 'Brake Pedal Sensor Disagreement',
    rule_ref: '— (dual-sensor)',
    description: 'Redundant brake pedal position sensors differ by more than 10% for over 200 ms.',
    sensors: ['s_06', 's_07'],
    severity: 'warning',
    timing_ms: 200,
    latch: false,
    latch_clear_fn: null,
    condition: (v) => Math.abs((v.s_06 ?? 0) - (v.s_07 ?? 0)) > 10,
  },
];
