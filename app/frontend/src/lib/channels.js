// Channel catalogue for the real TMS570 Main ECU telemetry stream (fed in
// via utils/tms570_bridge.py -- see that script and
// tms570_freertos_v2/app/main.c's vPackFrame() for the source frame
// format). Core channels (tps/front_axle/currents/rear) stream continuously
// once adc2can_front/rear are on the bus; the driver channels only appear
// once a motor driver node is actually transmitting on the bus, since the
// bridge omits fields for messages it has never seen.
export const CHANNELS = {
  tps1: { name: 'TPS 1', group: 'tps', unit: '', color: '#E57373' },
  tps2: { name: 'TPS 2', group: 'tps', unit: '', color: '#EF9A9A' },

  wheel_speed_fl: { name: 'Wheel Speed FL', group: 'front_axle', unit: '', color: '#4FC3F7' },
  wheel_speed_fr: { name: 'Wheel Speed FR', group: 'front_axle', unit: '', color: '#81D4FA' },
  steering_angle: { name: 'Steering Angle', group: 'front_axle', unit: '°', color: '#29B6F6' },
  front_brake:    { name: 'Front Brake',    group: 'front_axle', unit: '', color: '#0288D1' },

  ac_current_1: { name: 'AC Current 1', group: 'currents', unit: 'A', color: '#4DB6AC' },
  ac_current_2: { name: 'AC Current 2', group: 'currents', unit: 'A', color: '#80CBC4' },
  dc_current_1: { name: 'DC Current 1', group: 'currents', unit: 'A', color: '#26C6DA' },
  dc_current_2: { name: 'DC Current 2', group: 'currents', unit: 'A', color: '#80DEEA' },

  rear_brake: { name: 'Rear Brake', group: 'rear', unit: '', color: '#CE93D8' },

  drv1_motor_velocity:      { name: 'Driver 1 Motor Velocity', group: 'driver1', unit: 'rpm', color: '#FFB74D' },
  drv1_motor_rated_current: { name: 'Driver 1 Motor Current',  group: 'driver1', unit: 'A',   color: '#FFA726' },
  drv1_motor_temp:          { name: 'Driver 1 Motor Temp',     group: 'driver1', unit: '°C',  color: '#FB8C00' },
  drv1_driver_temp:         { name: 'Driver 1 Temp',           group: 'driver1', unit: '°C',  color: '#F57C00' },
  drv1_driver_dc_voltage:   { name: 'Driver 1 DC Bus Voltage', group: 'driver1', unit: 'V',   color: '#EF6C00' },
  drv1_status_word:         { name: 'Driver 1 Status Word',    group: 'driver1', unit: '',    color: '#E65100' },
  drv1_warnings:            { name: 'Driver 1 Warnings',       group: 'driver1', unit: '',    color: '#D84315' },
  drv1_error:               { name: 'Driver 1 Error',          group: 'driver1', unit: '',    color: '#BF360C' },
  drv1_control_word:        { name: 'Driver 1 Control Word',   group: 'driver1', unit: '',    color: '#8D6E63' },
  drv1_target_torque:       { name: 'Driver 1 Target Torque',  group: 'driver1', unit: '',    color: '#6D4C41' },
  drv1_heartbeat_state:     { name: 'Driver 1 Heartbeat',      group: 'driver1', unit: '',    color: '#5D4037' },

  drv2_motor_velocity:      { name: 'Driver 2 Motor Velocity', group: 'driver2', unit: 'rpm', color: '#BA68C8' },
  drv2_motor_rated_current: { name: 'Driver 2 Motor Current',  group: 'driver2', unit: 'A',   color: '#AB47BC' },
  drv2_motor_temp:          { name: 'Driver 2 Motor Temp',     group: 'driver2', unit: '°C',  color: '#9C27B0' },
  drv2_driver_temp:         { name: 'Driver 2 Temp',           group: 'driver2', unit: '°C',  color: '#8E24AA' },
  drv2_driver_dc_voltage:   { name: 'Driver 2 DC Bus Voltage', group: 'driver2', unit: 'V',   color: '#7B1FA2' },
  drv2_status_word:         { name: 'Driver 2 Status Word',    group: 'driver2', unit: '',    color: '#6A1B9A' },
  drv2_warnings:            { name: 'Driver 2 Warnings',       group: 'driver2', unit: '',    color: '#4A148C' },
  drv2_error:               { name: 'Driver 2 Error',          group: 'driver2', unit: '',    color: '#38006B' },
  drv2_control_word:        { name: 'Driver 2 Control Word',   group: 'driver2', unit: '',    color: '#5E35B1' },
  drv2_target_torque:       { name: 'Driver 2 Target Torque',  group: 'driver2', unit: '',    color: '#512DA8' },
  drv2_heartbeat_state:     { name: 'Driver 2 Heartbeat',      group: 'driver2', unit: '',    color: '#4527A0' },
};

export const GROUPS = {
  tps:        { label: 'TPS',            channels: ['tps1', 'tps2'] },
  front_axle: { label: 'Front Axle',     channels: ['wheel_speed_fl', 'wheel_speed_fr', 'steering_angle', 'front_brake'] },
  currents:   { label: 'ADC Currents',   channels: ['ac_current_1', 'ac_current_2', 'dc_current_1', 'dc_current_2'] },
  rear:       { label: 'Rear',           channels: ['rear_brake'] },
  driver1:    { label: 'Driver 1 (left)',  channels: ['drv1_motor_velocity', 'drv1_motor_rated_current', 'drv1_motor_temp', 'drv1_driver_temp', 'drv1_driver_dc_voltage', 'drv1_status_word', 'drv1_warnings', 'drv1_error', 'drv1_control_word', 'drv1_target_torque', 'drv1_heartbeat_state'] },
  driver2:    { label: 'Driver 2 (right)', channels: ['drv2_motor_velocity', 'drv2_motor_rated_current', 'drv2_motor_temp', 'drv2_driver_temp', 'drv2_driver_dc_voltage', 'drv2_status_word', 'drv2_warnings', 'drv2_error', 'drv2_control_word', 'drv2_target_torque', 'drv2_heartbeat_state'] },
};

export const ALL_CHANNEL_KEYS = Object.keys(CHANNELS);
