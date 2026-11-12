"""Telemetry v3 receiver: reads main-ECU frames from a serial port and logs them.

Scrolling log counterpart of v3view.py. Frame format and decoding live in
v3frame.py (a port of app/internal/serial/frame.go).
"""
import argparse
import logging
import time

import serial

from v3frame import HDR, LAYOUTS, TYPE_FAST, Parser, decode_payload, fmt_val

logger = logging.getLogger(__name__)

TYPE_NAMES = {TYPE_FAST: "fast", 2: "slow"}
STATS_PERIOD = 5.0  # s


def hex_dump(b):
    return ' '.join(f'{x:02X}' for x in b)


def main():
    parser = argparse.ArgumentParser(description="Telemetry v3 receiver")
    parser.add_argument('-p', '--port', type=str, default='/dev/ttyUSB0',
                        help='Serial port to listen on (default: /dev/ttyUSB0)')
    parser.add_argument('-b', '--baud', type=int, default=115200,
                        help='Baud rate (default: 115200)')
    parser.add_argument('-v', '--verbose', action='store_true',
                        help='Print every decoded field of each frame')
    parser.add_argument('-d', '--debug', action='store_true',
                        help='Enable debug logging (prints raw frames and rejected bytes)')
    args = parser.parse_args()

    logging.basicConfig(
        level=logging.DEBUG if args.debug else logging.INFO,
        format='%(asctime)s [%(levelname)s] %(message)s',
        datefmt='%H:%M:%S'
    )

    logger.info(f"Starting telemetry v3 receiver on {args.port} at {args.baud} baud...")
    p = Parser()
    counts = {}

    try:
        with serial.Serial(args.port, args.baud, timeout=0.1) as ser:
            logger.info("Device successfully opened and ready. Listening for data...")
            next_stats = time.monotonic() + STATS_PERIOD

            while True:
                data = ser.read(ser.in_waiting or 1)
                before = (p.discarded, p.rejected, p.crc_errors, p.lost)

                for typ, seq, t_ms, frame in p.feed(data) if data else []:
                    name = TYPE_NAMES.get(typ, f"type {typ}")
                    counts[name] = counts.get(name, 0) + 1
                    logger.debug(f"Raw {name} frame ({len(frame)} B): {hex_dump(frame)}")
                    if args.verbose and typ in LAYOUTS:
                        fields = decode_payload(typ, frame[HDR:-2])
                        print(f"\n--- {name} frame  seq {seq}  ecu_time {t_ms / 1000:.3f} s ---")
                        for key, (raw, val, scale) in fields.items():
                            print(f"  {key:<26} {fmt_val(key, raw, val, scale):>12}")

                if logger.isEnabledFor(logging.DEBUG):
                    d_disc, d_rej, d_crc, d_lost = (a - b for a, b in zip(
                        (p.discarded, p.rejected, p.crc_errors, p.lost), before))
                    if d_crc:
                        logger.debug(f"Dropped {d_crc} frame(s): CRC mismatch.")
                    if d_rej - d_crc:
                        logger.debug(f"Rejected {d_rej - d_crc} false sync(s).")
                    if d_disc - d_rej:
                        logger.debug(f"Discarded {d_disc - d_rej} byte(s) while hunting for sync.")
                    if d_lost:
                        logger.debug(f"Sequence gap: {d_lost} frame(s) lost.")

                now = time.monotonic()
                if now >= next_stats:
                    got = ', '.join(f"{n} {k}" for k, n in sorted(counts.items())) or "no frames"
                    logger.info(f"Link: {got} | crc_err {p.crc_errors} lost {p.lost} "
                                f"rejected {p.rejected} discarded {p.discarded} B")
                    next_stats = now + STATS_PERIOD

    except serial.SerialException as e:
        logger.error(f"Failed to open serial port: {e}")
    except KeyboardInterrupt:
        logger.info("Receiver stopped by user.")


if __name__ == '__main__':
    main()
