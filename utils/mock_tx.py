import serial
import msgpack
import argparse
import logging
import time
import math
import random
from cobs import cobs

logger = logging.getLogger(__name__)

NUM_SENSORS = 12

# ---------------------------------------------------------------------------
# Fault injection schedule
# ---------------------------------------------------------------------------
# Each entry: (fault_type, duration_s, interval_s)
# Faults fire at random within the interval window.
FAULT_DEFS = [
    ('apps_implausibility', 0.25, 30),   # s_04/s_05 diverge >10% for 250ms
    ('apps_brake',          0.20, 45),   # accel>25% while brake>10%
    ('high_power',          0.20, 40),   # all motors at 70 A
    ('brake_sensor_diff',   0.50, 35),   # s_00/s_01 disagree >0.5 bar
]

class FaultScheduler:
    def __init__(self, fault_defs, start_time):
        self.faults = []
        for (kind, dur, interval) in fault_defs:
            # First trigger: random offset within first interval
            first = start_time + random.uniform(interval * 0.4, interval * 0.9)
            self.faults.append({
                'kind': kind, 'dur': dur, 'interval': interval,
                'next_at': first, 'active_until': 0,
            })

    def active_faults(self, t):
        result = set()
        for f in self.faults:
            if t >= f['next_at'] and f['active_until'] == 0:
                f['active_until'] = t + f['dur']
                f['next_at'] = t + f['dur'] + f['interval'] + random.uniform(-5, 5)
                logger.info(f"  *** INJECTING FAULT: {f['kind']} for {f['dur']:.2f}s ***")
            if t < f['active_until']:
                result.add(f['kind'])
            elif f['active_until'] != 0 and t >= f['active_until']:
                f['active_until'] = 0
        return result


def generate_mock_data(t, faults):
    """
    Physics-aware sensor simulation with optional fault injection.
    All values are realistic for an FSAE EV car.
    """
    # --- Brake pressure: 0–80 bar, correlated, normally near 0 when accel is high ---
    accel_phase = math.sin(t * 0.3) * 0.5 + 0.5          # 0–1 driving cycle
    brake_base = max(0, -math.sin(t * 0.3)) * 60 + max(0, math.sin(t * 1.1 + 1.0)) * 20
    s_00 = max(0, brake_base + random.gauss(0, 0.3))
    s_01 = max(0, brake_base + random.gauss(0, 0.3))
    if 'brake_sensor_diff' in faults:
        s_01 += 1.2  # 1.2 bar disagreement

    # --- Battery level: starts ~90%, drains slowly ---
    s_02 = max(0, 90 - t * 0.008 + random.gauss(0, 0.05))

    # --- Steering angle: ±90° sinusoidal cornering ---
    s_03 = math.sin(t * 0.18) * 90 + math.sin(t * 0.07) * 20

    # --- Accel pedal: 0–100%, drives with the brake inverse ---
    accel_raw = max(0, min(100, accel_phase * 85 + math.sin(t * 0.8) * 10))
    s_04 = round(accel_raw + random.gauss(0, 0.5), 2)
    s_05 = round(accel_raw + random.gauss(0, 0.5), 2)
    if 'apps_implausibility' in faults:
        s_05 += 15   # >10% divergence
    if 'apps_brake' in faults:
        s_04 = 35
        s_05 = 35

    # --- Brake pedal: 0–100%, inverse to accel ---
    brake_ped_raw = max(0, min(100, (1 - accel_phase) * 60 + max(0, math.sin(t * 1.1 + 1.0)) * 30))
    s_06 = round(brake_ped_raw + random.gauss(0, 0.5), 2)
    s_07 = round(brake_ped_raw + random.gauss(0, 0.5), 2)
    if 'apps_brake' in faults:
        s_06 = 50
        s_07 = 50

    # --- Motor currents: proportional to accel, 0–60 A, balanced ±noise ---
    motor_base = accel_raw * 0.6   # 0–60 A
    s_08 = round(motor_base + random.gauss(0, 1.0), 2)
    s_09 = round(motor_base + random.gauss(0, 1.0), 2)
    s_10 = round(motor_base + random.gauss(0, 1.0), 2)
    s_11 = round(motor_base + random.gauss(0, 1.0), 2)
    if 'high_power' in faults:
        s_08 = s_09 = s_10 = s_11 = 70  # 4 × 70 A × 400 V = 112 kW > 80 kW limit

    return {
        's_00': round(s_00, 2), 's_01': round(s_01, 2),
        's_02': round(s_02, 2),
        's_03': round(s_03, 2),
        's_04': round(s_04, 2), 's_05': round(s_05, 2),
        's_06': round(s_06, 2), 's_07': round(s_07, 2),
        's_08': round(s_08, 2), 's_09': round(s_09, 2),
        's_10': round(s_10, 2), 's_11': round(s_11, 2),
    }


def send_packet(ser, data_dict):
    try:
        msgpack_payload = msgpack.packb(data_dict, use_bin_type=True)
        cobs_encoded = cobs.encode(msgpack_payload)
        frame = cobs_encoded + b'\x00'
        ser.write(frame)
        if logger.isEnabledFor(logging.DEBUG):
            hex_dump = ' '.join(f'{b:02X}' for b in frame)
            logger.debug(f"Transmitted raw frame: {hex_dump}")
        return True
    except Exception as e:
        logger.error(f"Failed to send packet: {e}")
        return False


def main():
    parser = argparse.ArgumentParser(description="Framed Msgpack Telemetry Transmitter (Mock MCU)")
    parser.add_argument('-p', '--port', type=str, default='/dev/ttyUSB1',
                        help='Serial port to transmit on (default: /dev/ttyUSB1)')
    parser.add_argument('-b', '--baud', type=int, default=115200,
                        help='Baud rate (default: 115200)')
    parser.add_argument('-r', '--rate', type=float, default=10.0,
                        help='Transmission rate in Hz (default: 10.0 Hz)')
    parser.add_argument('-v', '--verbose', action='store_true',
                        help='Print formatted sensor data as it is sent')
    parser.add_argument('-d', '--debug', action='store_true',
                        help='Enable debug logging (prints raw bytes)')
    parser.add_argument('--inject-faults', action='store_true',
                        help='Periodically inject FSAE rule violations for testing')

    args = parser.parse_args()

    log_level = logging.DEBUG if args.debug else logging.INFO
    logging.basicConfig(
        level=log_level,
        format='%(asctime)s [%(levelname)s] %(message)s',
        datefmt='%H:%M:%S',
    )

    logger.info(f"Starting Mock MCU Transmitter on {args.port} at {args.baud} baud ({args.rate} Hz)...")
    if args.inject_faults:
        logger.info("Fault injection ENABLED — violations will fire every ~30–45 s")

    sleep_time = 1.0 / args.rate
    start = time.monotonic()
    scheduler = FaultScheduler(FAULT_DEFS, start) if args.inject_faults else None

    try:
        with serial.Serial(args.port, args.baud, timeout=1) as ser:
            logger.info("Device successfully opened. Starting transmission loop...")

            while True:
                t = time.monotonic() - start
                faults = scheduler.active_faults(t) if scheduler else set()

                mock_data = generate_mock_data(t, faults)
                success = send_packet(ser, mock_data)

                if success and args.verbose:
                    print("\n--- Transmitted Mock Packet ---")
                    for key, val in mock_data.items():
                        print(f"  {key}: {val:.2f}")

                time.sleep(sleep_time)

    except serial.SerialException as e:
        logger.error(f"Failed to open serial port: {e}")
    except KeyboardInterrupt:
        logger.info("Transmitter stopped by user.")


if __name__ == '__main__':
    main()
