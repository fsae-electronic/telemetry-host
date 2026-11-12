import serial
import struct
import argparse
import logging
import time
import math

logger = logging.getLogger(__name__)

# Telemetry frames v3, as the main ECU sends them (tms570_main_ecu,
# app/telemetry.c; spec "Frame de telemetria v3"). Must match
# app/internal/serial/frame.go. Little-endian. Header:
#   sync AA 55, version 3, type (1 fast / 2 slow), seq (per type),
#   payload_len, ecu_time_ms
# then the payload and a CRC-16/CCITT-FALSE over bytes 2 .. end of payload.
#
# Like the real ECU, fields it has no source for yet are 0 (BMS, IMU,
# coolant, LV, tractive system, torque/current actual, ...), and steering
# and the currents are raw CAN values. The host's built-in SIMULATOR port
# exercises every field instead.
HEADER_FMT = '<2sBBBBI'
FAST_PAYLOAD_FMT = ('<' + 'IIHHHBB' + 'HHHHh' + 'HHHH' + 'hhh' + 'Hh' + 'Bb' + 'HHHH'
                    + 'hhhh' + 'hhh' + 'bB' + 'H' + 'HHHHhhhiHBBB' * 2)
SLOW_PAYLOAD_FMT = '<' + 'BBBB' + 'I' + 'I' + 'H' + 'BB' + 'HH' + 'I' + 'BB' + 'H' + 'II' + 'II' + 'BBBB'
TYPE_FAST, TYPE_SLOW = 1, 2

assert struct.calcsize(HEADER_FMT) + struct.calcsize(FAST_PAYLOAD_FMT) + 2 == 130
assert struct.calcsize(HEADER_FMT) + struct.calcsize(SLOW_PAYLOAD_FMT) + 2 == 60

GEAR_RATIO = 4.0
BOOT_MESSAGE = b"TMS570 MAIN ECU (rewrite) INITIALIZED\n"


def crc16(data):
    """CRC-16/CCITT-FALSE. crc16(b'123456789') == 0x29B1."""
    crc = 0xFFFF
    for b in data:
        crc ^= b << 8
        for _ in range(8):
            crc = ((crc << 1) ^ 0x1021) & 0xFFFF if crc & 0x8000 else (crc << 1) & 0xFFFF
    return crc


def frame(frame_type, seq, ecu_time_ms, payload):
    body = struct.pack(HEADER_FMT, b'\xAA\x55', 3, frame_type, seq & 0xFF, len(payload),
                       ecu_time_ms & 0xFFFFFFFF) + payload
    return body + struct.pack('<H', crc16(body[2:]))


def wave(t, phase, amp, offset):
    return int(round(math.sin(t + phase) * amp + offset))


def driver_block(t, phase, fault):
    velocity = max(0, wave(t, phase, 2000, 2000))
    return (
        0x000F,                          # control_word (what the ECU sends)
        0x0218 if fault else 0x0637,     # status_word: CiA 402 Fault / Operation enabled
        0,                               # warnings
        0x2310 if fault else 0,          # error: continuous over-current
        wave(t, phase, 300, 300),        # target_torque, ‰
        0,                               # torque_actual (not mapped yet)
        0,                               # current_actual (not mapped yet)
        velocity,                        # motor_velocity, rpm
        wave(t / 5, phase, 20, 960),     # dc_voltage, 0.1 V
        wave(t / 10, phase, 3, 25),      # driver_temp, °C
        wave(t / 10, phase, 3, 22),      # motor_temp, °C
        5,                               # heartbeat_state: NMT operational
    ), velocity


def build_fast(t, faults):
    """One fast frame payload with smoothly oscillating values.

    With faults, every 20 s it injects 1 s of TPS implausibility and then a
    2 s fault on driver L, to exercise the Health tab and event log.
    """
    cycle = t % 20
    tps_fault = faults and 5 <= cycle < 6
    drv_fault = faults and 12 <= cycle < 14
    tps1 = max(0, wave(t, 0.0, 50, 50))                       # %
    tps2 = max(0, tps1 - 25) if tps_fault else max(0, wave(t, 0.1, 50, 50))
    tc = 0.7 if math.sin(t / 3) > 0.5 else 1.0                # traction control trims the right side
    request = tps1 * 10                                       # race map, ‰
    drv_l, vel_l = driver_block(t, 0.0, drv_fault)
    drv_r, vel_r = driver_block(t, 0.4, False)

    ecu_state = 1 | 1 << 2 | 1 << 4                           # race, drive, TC on
    if tc < 1:
        ecu_state |= 1 << 6
    limit = 1 << 3 if tc < 1 else 0
    if drv_fault:
        limit |= 1 << 11                                      # driver not enabled

    values = (
        0x17FFF, 0x17FFF,           # valid_mask, fresh_mask: bits 0–14 and rear wheels (16)
        ecu_state, limit, 0, 1, 0,  # ecu_state, torque_limit_reason, ts_state, calibration_type, sdc_open_segment
        tps1 * 10, tps2 * 10,       # 0.1 %
        wave(t, 1.5, 15, 17),       # front_brake, PSI
        wave(t, 1.7, 30, 36),       # rear_brake, PSI
        wave(t, 1.0, 3, 2024),      # steering_angle, raw ADC
        max(0, wave(t, 0.5, 3000, 3000)), max(0, wave(t, 0.6, 3000, 3000)),  # wheel fl/fr, 0.1 rpm
        int(vel_l / GEAR_RATIO * 10), int(vel_r / GEAR_RATIO * 10),         # wheel rl/rr from motors
        request, request, int(request * tc),                                 # torque request, cmd l/r
        0, 0, 0, 0, 0, 0, 0, 0,     # BMS block: no source yet
        max(0, wave(t, 2.5, 8, 8)), max(0, wave(t, 2.7, 8, 8)),             # dc_current l/r, raw
        max(0, wave(t, 2.0, 6, 6)), max(0, wave(t, 2.2, 6, 6)),             # ac_current l/r, raw
        0, 0, 0, 0, 0, 0,           # IMU, coolant, reserved, LV: no source yet
    ) + drv_l + drv_r
    return struct.pack(FAST_PAYLOAD_FMT, *values)


def build_slow(t):
    values = (
        0, 3, 0, 1,                 # firmware 0.3.0, dirty
        0x0885E466,                 # git_hash
        int(t),                     # uptime_s
        0, 0,                       # reset_count, last_reset_cause: not stored yet
        wave(t, 0, 5, 35),          # cpu_load, %
        0, wave(t, 1, 80, 900),     # loop_overruns, loop_time_max (µs)
        0,                          # active_faults
        0, 0, 0,                    # CAN TEC, REC, bus-off count
        200000, 200000,             # motor_rated_current l/r, mA
        0, 0,                       # motor_rated_torque l/r: not read yet
        0, 0, 0, 0,                 # cell ids, reserved
    )
    return struct.pack(SLOW_PAYLOAD_FMT, *values)


def main():
    parser = argparse.ArgumentParser(description="Telemetry v3 transmitter (mock main ECU)")
    parser.add_argument('-p', '--port', type=str, default='/dev/ttyUSB1',
                        help='Serial port to transmit on (default: /dev/ttyUSB1)')
    parser.add_argument('-b', '--baud', type=int, default=115200,
                        help='Baud rate (default: 115200)')
    parser.add_argument('-r', '--rate', type=float, default=10.0,
                        help='Fast frame rate in Hz (default: 10.0 Hz); the slow frame goes out at 1 Hz')
    parser.add_argument('-d', '--debug', action='store_true',
                        help='Enable debug logging (prints raw bytes)')
    parser.add_argument('-f', '--faults', action='store_true',
                        help='Periodically inject TPS implausibility and a driver fault')

    args = parser.parse_args()

    logging.basicConfig(
        level=logging.DEBUG if args.debug else logging.INFO,
        format='%(asctime)s [%(levelname)s] %(message)s',
        datefmt='%H:%M:%S'
    )

    logger.info(f"Starting mock main ECU on {args.port} at {args.baud} baud ({args.rate} Hz)...")

    period = 1.0 / args.rate
    slow_every = max(1, round(args.rate))
    start = time.monotonic()
    fast_seq = slow_seq = n = 0

    try:
        with serial.Serial(args.port, args.baud, timeout=1) as ser:
            logger.info("Device successfully opened. Starting transmission loop...")
            ser.write(BOOT_MESSAGE)
            while True:
                t = n * period
                ms = int(t * 1000)
                out = frame(TYPE_FAST, fast_seq, ms, build_fast(t, args.faults))
                fast_seq += 1
                if n % slow_every == 0:
                    out += frame(TYPE_SLOW, slow_seq, ms, build_slow(t))
                    slow_seq += 1
                ser.write(out)
                if logger.isEnabledFor(logging.DEBUG):
                    logger.debug("TX " + ' '.join(f'{b:02X}' for b in out))
                n += 1
                time.sleep(max(0.0, start + n * period - time.monotonic()))

    except serial.SerialException as e:
        logger.error(f"Failed to open serial port: {e}")
    except KeyboardInterrupt:
        logger.info("Transmitter stopped by user.")


if __name__ == '__main__':
    main()
