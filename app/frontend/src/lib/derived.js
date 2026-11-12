// Channels calculated on the host from the frame. The same code runs on
// live packets and on saved runs, so both always agree.

// driver_dc_voltage arrives in volts. The main ECU still sends dc_current_1/2
// raw (unit unknown, 1 = left, 2 = right), so per-driver power assumes 1 A per
// unit until it sends amperes. When the BMS block is valid, total power and
// energy come from the pack voltage and current instead.
export const DC_CURRENT_SCALE = 1;    // A per unit (unconfirmed)
const VALID_BMS = 1 << 15;

// FSAE T.4.2: the two APPS may not differ by more than 10 points of pedal
// travel for more than 100 ms. Host timestamps jitter by a few tens of ms, so
// 100 ms here means "seen on two consecutive 10 Hz frames".
export const TPS_MAX_DIFF = 10;       // %
export const TPS_MAX_DURATION = 0.1;  // s
const TS_JITTER = 0.03;               // s

// Energy is not integrated across gaps longer than this.
const ENERGY_MAX_GAP = 1;             // s

export const DERIVED_CHANNELS = {
  tps_diff:        { name: 'TPS Diff',        unit: '%',  color: '#FFE082' },
  tps_implausible: { name: 'TPS Implausible', unit: '',   color: '#FF5252' },
  power_l:         { name: 'DC Power L',      unit: 'kW', color: '#AED581' },
  power_r:         { name: 'DC Power R',      unit: 'kW', color: '#C5E1A5' },
  power_total:     { name: 'DC Power Total',  unit: 'kW', color: '#7CB342' },
  energy:          { name: 'Energy (net)',    unit: 'Wh', color: '#FFD54F' },
  frame_dt:        { name: 'Frame Interval',  unit: 'ms', color: '#90A4AE' },
};

function dcPower(voltage, current) {
  if (voltage == null || current == null) return null;
  return voltage * current * DC_CURRENT_SCALE / 1000;
}

// createDeriver returns derive(ts, data) → {derived channel: value}. It keeps
// state between calls (energy, persistence timers), so feed it the samples of
// one session in time order and make a new one per session.
export function createDeriver() {
  let prevTs = null;
  let prevEcuTime = null;
  let prevPower = null;
  let energy = 0;          // Wh
  let diffSince = null;    // ts when the current TPS deviation started

  return function derive(ts, d) {
    const out = {};

    if (d.tps1 != null && d.tps2 != null) {
      const diff = Math.abs(d.tps1 - d.tps2);
      if (diff > TPS_MAX_DIFF) diffSince ??= ts;
      else diffSince = null;
      out.tps_diff = diff;
      out.tps_implausible =
        diffSince != null && ts - diffSince >= TPS_MAX_DURATION - TS_JITTER ? 1 : 0;
    } else {
      out.tps_diff = null;
      out.tps_implausible = null;
    }

    const pl = dcPower(d.drv0_driver_dc_voltage, d.dc_current_1);
    const pr = dcPower(d.drv1_driver_dc_voltage, d.dc_current_2);
    const total = (d.valid_mask & VALID_BMS) && d.pack_voltage != null && d.pack_current != null
      ? d.pack_voltage * d.pack_current / 1000
      : pl == null && pr == null ? null : (pl ?? 0) + (pr ?? 0);
    out.power_l = pl;
    out.power_r = pr;
    out.power_total = total;

    // Integrate on the ECU's clock (when the data was sampled) if there is
    // one, otherwise on arrival time.
    const dt = prevTs == null ? null : ts - prevTs;
    const dtEnergy = d.ecu_time != null && prevEcuTime != null ? d.ecu_time - prevEcuTime : dt;
    if (dtEnergy != null && dtEnergy > 0 && dtEnergy <= ENERGY_MAX_GAP && total != null && prevPower != null) {
      energy += (prevPower + total) / 2 * dtEnergy / 3.6;  // kW·s = kJ; 3.6 kJ = 1 Wh
    }
    out.energy = energy;
    out.frame_dt = dt == null ? null : dt * 1000;

    prevTs = ts;
    prevEcuTime = d.ecu_time ?? null;
    prevPower = total;
    return out;
  };
}
